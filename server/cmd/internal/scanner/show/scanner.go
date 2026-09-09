package show

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

type Dependencies struct {
	Now                   func() time.Time
	DB                    *sql.DB
	Queries               *database.Queries
	Logger                logger.LoggerInterface
	Ffprobe               ffprobe.FfprobeInterface
	Tmdb                  tmdb.TmdbInterface
	ScanContext           context.Context
	Wait                  *sync.WaitGroup
	ScannerDBMu           *sync.Mutex
	CurrentShowsDirectory func() sql.NullString
}
type Scanner struct {
	Dependencies
	guard scanner.ScanGuard
}
type StartStatus int

const (
	StartStarted StartStatus = iota
	StartNotConfigured
	StartAlreadyRunning
)

type StartResult struct {
	Directory string
	Status    StartStatus
}

func New(deps Dependencies) *Scanner {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.ScanContext == nil {
		deps.ScanContext = context.Background()
	}
	if deps.Wait == nil {
		deps.Wait = &sync.WaitGroup{}
	}
	if deps.ScannerDBMu == nil {
		deps.ScannerDBMu = &sync.Mutex{}
	}
	if deps.CurrentShowsDirectory == nil {
		deps.CurrentShowsDirectory = func() sql.NullString { return sql.NullString{} }
	}
	return &Scanner{Dependencies: deps}
}
func (s *Scanner) Start() StartResult {
	directory := s.CurrentShowsDirectory()
	result := StartResult{Directory: directory.String}
	if !directory.Valid || directory.String == "" {
		result.Status = StartNotConfigured
		return result
	}
	began := s.guard.TryBegin()
	if !began {
		result.Status = StartAlreadyRunning
		return result
	}
	s.Wait.Add(1)
	go func() { defer s.Wait.Done(); defer s.guard.Finish(); s.run(directory.String) }()
	return result
}

type scanState struct {
	index                                                             map[string]database.GetShowScanIndexRow
	seasons                                                           map[int64]bool
	episodes                                                          map[int64]bool
	imported, updated, skipped, deferred, rejected, deleted, enriched int
}

func (s *Scanner) run(directory string) {
	err := s.scan(directory)
	if err != nil {
		contextErr := s.ScanContext.Err()
		if contextErr != nil {
			s.Logger.Info("show scan interrupted", "error", err)
		} else {
			s.Logger.Error("show scan failed", "error", err)
		}
	}
}
func (s *Scanner) scan(directory string) error {
	ctx := s.ScanContext
	root, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	s.Logger.Info("scanning shows directory", "path", root)
	rows, err := s.Queries.GetShowScanIndex(ctx)
	if err != nil {
		return err
	}
	state := &scanState{index: make(map[string]database.GetShowScanIndexRow), seasons: make(map[int64]bool), episodes: make(map[int64]bool)}
	files := make([]scanner.CatalogFile, 0, len(rows))
	for _, row := range rows {
		state.index[filepath.Clean(row.FilePath)] = row
		files = append(files, scanner.CatalogFile{ID: row.ID, Path: row.FilePath})
	}
	reconciliation, err := scanner.NewReconciliation(root, files)
	if err != nil {
		return err
	}
	batch := make([]scanner.ScanFile, 0, scanner.BatchSize)
	flush := func() error {
		for _, file := range batch {
			err := s.processFile(ctx, state, root, file)
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
			var deferred *scanner.FileDeferral
			isDeferred := errors.As(err, &deferred)
			if isDeferred {
				state.deferred++
				s.Logger.Debug("deferred show file", "path", file.Path, "error", err)
			} else if err != nil {
				state.rejected++
				s.Logger.Warn("rejected show file", "path", file.Path, "error", err)
			}
		}
		batch = batch[:0]
		return nil
	}
	err = scanner.WalkMediaLibraryContext(ctx, root, helpers.ValidVideoExtensions, func(err error) { s.Logger.Warn("show walk error", "error", err) }, func(file scanner.ScanFile) error {
		reconciliation.MarkSeen(file.Path)
		hidden := strings.HasPrefix(filepath.Base(file.Path), ".")
		if hidden {
			state.skipped++
			return nil
		}
		batch = append(batch, file)
		if len(batch) >= scanner.BatchSize {
			return flush()
		}
		return nil
	}, func(path string) bool { return allowDirectory(root, path) })
	if err != nil {
		return err
	}
	err = flush()
	if err != nil {
		return err
	}
	err = s.cleanup(ctx, reconciliation, state)
	if err != nil {
		return err
	}
	err = s.enrich(ctx, state)
	if err != nil {
		return err
	}
	pending, err := s.Queries.CountShowRetries(ctx)
	if err != nil {
		return err
	}
	s.Logger.Info("show scan completed", "files_imported", state.imported, "files_updated", state.updated, "files_skipped", state.skipped, "files_deferred", state.deferred, "files_rejected", state.rejected, "files_deleted", state.deleted, "local_episodes_processed", len(state.episodes), "entities_enriched", state.enriched, "entities_pending", pending)
	return nil
}

func (s *Scanner) processFile(ctx context.Context, state *scanState, root string, file scanner.ScanFile) (err error) {
	local, err := parseFile(root, file.Path)
	if err != nil {
		return err
	}
	baseline, exists := state.index[file.Path]
	var previous *scanner.FileFingerprint
	if exists && baseline.MtimeNs.Valid {
		previous = &scanner.FileFingerprint{Size: baseline.Size, MtimeNS: baseline.MtimeNs.Int64, CtimeNS: baseline.CtimeNs.Int64, Device: baseline.Device.String, Inode: baseline.Inode.String}
		copy(previous.SHA256[:], baseline.Sha256)
	}
	inspection, err := scanner.InspectFile(ctx, file.Path, previous, s.Now)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, inspection.Close()) }()
	if inspection.Outcome == scanner.FileDeferred {
		state.deferred++
		s.Logger.Debug("deferred show file", "path", file.Path, "reason", inspection.Reason, "eligible_at", inspection.EligibleAt)
		return nil
	}
	id := baseline.ID
	if inspection.Outcome == scanner.FileUnchanged {
		state.skipped++
	} else {
		var info *ffprobe.FfprobeResult
		if inspection.Outcome == scanner.FileNeedsProcessing {
			info, err = s.Ffprobe.GetMetadata(ctx, file.Path)
			if err != nil {
				return err
			}
			if info == nil {
				return errors.New("ffprobe returned no metadata")
			}
		}
		id, err = s.persistFile(ctx, local, file, baseline, inspection, info)
		if err != nil {
			return err
		}
		if id == 0 {
			return nil
		}
		switch inspection.Outcome {
		case scanner.FileFingerprintOnly:
			state.skipped++
		default:
			if exists {
				state.updated++
			} else {
				state.imported++
			}
		}
	}
	episodes, err := s.Queries.GetShowFileEpisodes(ctx, id)
	if err != nil {
		return err
	}
	for _, episode := range episodes {
		state.seasons[episode.SeasonID] = true
		state.episodes[episode.ID] = true
	}
	return nil
}

func (s *Scanner) persistFile(ctx context.Context, local localEpisodeFile, file scanner.ScanFile, baseline database.GetShowScanIndexRow, inspection *scanner.FileInspection, info *ffprobe.FfprobeResult) (int64, error) {
	s.ScannerDBMu.Lock()
	defer s.ScannerDBMu.Unlock()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	current, err := q.GetShowFileByPath(ctx, file.Path)
	missing := errors.Is(err, sql.ErrNoRows)
	if err != nil && !missing {
		return 0, err
	}
	if current.ID != baseline.ID || current.FilePath != baseline.FilePath {
		return 0, nil
	}
	id := current.ID
	if info != nil {
		show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{DirectoryPath: local.showPath, LocalName: local.title, PremiereYear: helpers.NullInt64(int64(local.year)), Name: local.title})
		if err != nil {
			return 0, err
		}
		season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{ShowID: show.ID, SeasonNumber: int64(local.season), Name: fmt.Sprintf("Season %d", local.season)})
		if err != nil {
			return 0, err
		}
		if current.ID != 0 && current.SeasonID != season.ID {
			return 0, errors.New("file season ownership changed")
		}
		var duration sql.NullFloat64
		seconds, parseErr := strconv.ParseFloat(info.Format.Duration, 64)
		validDuration := parseErr == nil && seconds > 0 && !math.IsInf(seconds, 0) && !math.IsNaN(seconds)
		if validDuration {
			duration = helpers.NullFloat64(seconds)
		}
		stored, err := q.UpsertShowFile(ctx, database.UpsertShowFileParams{SeasonID: season.ID, FilePath: file.Path, FileName: filepath.Base(file.Path), Size: inspection.Fingerprint.Size, Container: file.Ext, MimeType: helpers.VideoMimeTypes[file.Ext], Duration: duration})
		if err != nil {
			return 0, err
		}
		id = stored.ID
		count, err := s.processStreams(ctx, q, id, info.Streams)
		if err != nil {
			return 0, err
		}
		if count == 0 {
			return 0, errors.New("no accepted video stream")
		}
		err = processChapters(ctx, q, id, info.Chapters)
		if err != nil {
			return 0, err
		}
		err = q.DeleteShowFileLinks(ctx, id)
		if err != nil {
			return 0, err
		}
		for order, number := range local.episodes {
			ep, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{SeasonID: season.ID, EpisodeNumber: int64(number), Name: fmt.Sprintf("Episode %d", number)})
			if err != nil {
				return 0, err
			}
			err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{EpisodeID: ep.ID, FileID: id, SeasonID: season.ID, EpisodeOrder: int64(order)})
			if err != nil {
				return 0, err
			}
			err = q.MarkShowEpisodeRetry(ctx, ep.ID)
			if err != nil {
				return 0, err
			}
		}
		err = q.MarkShowRetry(ctx, show.ID)
		if err != nil {
			return 0, err
		}
		err = q.MarkShowSeasonRetry(ctx, season.ID)
		if err != nil {
			return 0, err
		}
		err = q.PruneShowEpisodes(ctx)
		if err != nil {
			return 0, err
		}
	}
	f := inspection.Fingerprint
	err = q.UpsertShowFingerprint(ctx, database.UpsertShowFingerprintParams{FileID: id, MtimeNs: f.MtimeNS, CtimeNs: f.CtimeNS, Device: f.Device, Inode: f.Inode, Sha256: f.SHA256[:]})
	if err != nil {
		return 0, err
	}
	err = inspection.Validate(ctx)
	if err != nil {
		return 0, err
	}
	err = tx.Commit()
	return id, err
}

func (s *Scanner) cleanup(ctx context.Context, r *scanner.Reconciliation, state *scanState) error {
	err := r.ValidateRoot(ctx)
	if err != nil {
		return err
	}
	for _, file := range r.Unseen() {
		deleted, err := s.deleteMissing(ctx, r, file)
		if err != nil {
			return err
		}
		if deleted {
			state.deleted++
		}
	}
	return ctx.Err()
}
func (s *Scanner) deleteMissing(ctx context.Context, r *scanner.Reconciliation, file scanner.CatalogFile) (bool, error) {
	missing, err := r.ConfirmMissing(ctx, file)
	if err != nil || !missing {
		return false, err
	}
	s.ScannerDBMu.Lock()
	defer s.ScannerDBMu.Unlock()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	q := s.Queries.WithTx(tx)
	count, err := q.DeleteMissingShowFile(ctx, database.DeleteMissingShowFileParams{ID: file.ID, FilePath: file.Path})
	if err != nil || count == 0 {
		return false, err
	}
	err = q.PruneShowEpisodes(ctx)
	if err != nil {
		return false, err
	}
	err = q.PruneShowSeasons(ctx)
	if err != nil {
		return false, err
	}
	err = q.PruneShows(ctx)
	if err != nil {
		return false, err
	}
	missing, err = r.ConfirmMissing(ctx, file)
	if err != nil || !missing {
		return false, err
	}
	err = tx.Commit()
	return err == nil, err
}
