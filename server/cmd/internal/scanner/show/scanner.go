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

const (
	progressLogInterval = 10 * time.Second
	// TV defers a changing file for the run; the next scan retries it.
	reasonDeferred   = "The file is still changing or has not been quiet for 60 seconds. It is retried on the next scan."
	reasonRejected   = "The file could not be read or its name does not identify a season and episode. Existing records are preserved."
	reasonEnrichment = "TMDB metadata could not be applied. The local entry is usable and the next scan retries it."
)

// errStaleCatalogRow aborts a persistence transaction without reporting a
// failure: the catalog row this scan resolved is no longer the row at that
// path, so the next scan owns it.
var errStaleCatalogRow = errors.New("show catalog row changed during the scan")

// Scanner scans and persists the configured TV library.
type Scanner struct {
	Dependencies
	launcher scanner.Launcher
	tx       scanner.TxRunner
	statusMu sync.RWMutex
	status   Status
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
	return &Scanner{
		Dependencies: deps,
		launcher:     scanner.Launcher{Wait: deps.Wait},
		tx:           scanner.TxRunner{DB: deps.DB, Mu: deps.ScannerDBMu, Queries: deps.Queries},
	}
}

// Start launches a scan asynchronously when configured and no show scan is running.
func (s *Scanner) Start() scanner.StartResult {
	return s.launcher.Launch(s.CurrentShowsDirectory(), s.beginReport, s.runShowScan)
}

// runShowScan expects beginReport to have published the run it continues.
func (s *Scanner) runShowScan(directory string) {
	report := newScanReport(s.Status())
	ctx := s.ScanContext
	stopProgressLog := scanner.StartProgressLog(progressLogInterval, func() {
		status := s.Status()
		s.Logger.Info("show scan progress", "run", status.RunID, "phase", status.Phase, "processed", status.Processed, "total", status.Total, "enriched", status.Enriched)
	})
	defer func() {
		stopProgressLog()
		contextErr := ctx.Err()
		if contextErr != nil {
			s.Logger.Info("show library scan interrupted", "phase", report.status.Phase, "error", contextErr)
		}
		report.Finish(&report.status.Progress, contextErr != nil)
		s.publish(report)
		now := *report.status.FinishedAt
		s.Logger.Info("show scan finished", "run", report.status.RunID, "state", report.status.State, "elapsed", now.Sub(*report.status.StartedAt), "processed", report.status.Processed, "total", report.status.Total, "imported", report.status.Imported, "updated", report.status.Updated, "unchanged", report.status.Unchanged, "failed", report.status.Failed, "deferred", report.status.Deferred, "deleted", report.status.Deleted, "episodes", report.status.Episodes, "enriched", report.status.Enriched, "enrichment_failed", report.status.EnrichmentFailed, "pending_enrichment", report.status.PendingEnrichment)
	}()
	// fail marks a fatal error; a cancellation is reported once by the deferred
	// finish instead.
	fail := func(err error, reason string) {
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		s.Logger.Error("show scan failed", "phase", report.status.Phase, "error", err)
		report.status.State = scanner.StateFailed
		report.Issue("", report.status.Phase, reason)
	}

	root, err := filepath.Abs(directory)
	if err != nil {
		fail(err, "The library directory is unavailable.")
		return
	}
	s.Logger.Info("show scan phase", "run", report.status.RunID, "phase", scanner.PhaseDiscovery, "directory", root)

	rows, err := s.Queries.GetShowScanIndex(ctx)
	if err != nil {
		fail(err, "Unable to read the TV catalog.")
		return
	}
	index := make(map[string]database.GetShowScanIndexRow, len(rows))
	files := make([]scanner.CatalogFile, 0, len(rows))
	for _, row := range rows {
		index[filepath.Clean(row.FilePath)] = row
		files = append(files, scanner.CatalogFile{ID: row.ID, Path: row.FilePath})
	}
	report.scan = newShowScanContext(index)

	reconciliation, err := scanner.NewReconciliation(ctx, root, files)
	if err != nil {
		fail(err, "The library directory is unavailable.")
		return
	}

	batch := make([]scanner.ScanFile, 0, scanner.BatchSize)
	flush := func() {
		for _, file := range batch {
			s.phase(report, scanner.PhaseLocal)
			report.Activate(file.Path)
			err := s.processFile(ctx, report, root, file)
			report.Deactivate(file.Path)
			report.status.Processed++
			if ctx.Err() != nil {
				return
			}
			var deferred *scanner.FileDeferral
			if errors.As(err, &deferred) {
				report.status.Deferred++
				report.Issue(file.Path, scanner.PhaseLocal, reasonDeferred)
				s.Logger.Debug("deferred show file", "path", file.Path, "error", err)
			} else if err != nil {
				report.status.Failed++
				report.Issue(file.Path, scanner.PhaseLocal, reasonRejected)
				s.Logger.Warn("rejected show file", "path", file.Path, "error", err)
			}
			s.publish(report)
		}
		batch = batch[:0]
	}

	err = scanner.WalkMediaLibraryContext(ctx, root, helpers.ValidVideoExtensions, func(path string, err error) {
		s.Logger.Warn("show walk error", "path", path, "error", err)
		report.Issue(path, scanner.PhaseDiscovery, scanner.ReasonDiscoveryEntry)
	}, func(file scanner.ScanFile) error {
		reconciliation.MarkSeen(file.Path)
		if strings.HasPrefix(filepath.Base(file.Path), ".") {
			return nil
		}
		report.status.Total++
		batch = append(batch, file)
		if len(batch) >= scanner.BatchSize {
			flush()
		}
		return ctx.Err()
	}, func(path string) bool { return allowDirectory(root, path) })
	if err != nil {
		fail(err, "The library directory could not be read.")
		return
	}
	flush()
	if ctx.Err() != nil {
		return
	}

	s.phase(report, scanner.PhaseCleanup)
	err = s.cleanup(ctx, reconciliation, report)
	if err != nil {
		fail(err, "Missing TV files could not be reconciled.")
		return
	}
	if ctx.Err() != nil {
		return
	}

	s.phase(report, scanner.PhaseEnrichment)
	err = s.enrich(ctx, report)
	if err != nil {
		fail(err, "TV metadata could not be updated.")
		return
	}

	pending, err := s.Queries.CountShowRetries(ctx)
	if err != nil {
		fail(err, "Unable to read pending TV enrichment.")
		return
	}
	report.status.PendingEnrichment = int(pending)
	s.publish(report)
}

func (s *Scanner) processFile(ctx context.Context, report *scanReport, root string, file scanner.ScanFile) (err error) {
	local, err := parseFile(root, file.Path)
	if err != nil {
		return err
	}
	baseline, exists := report.scan.index[file.Path]
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
		return &scanner.FileDeferral{Reason: inspection.Reason, EligibleAt: inspection.EligibleAt}
	}
	id := baseline.ID
	if inspection.Outcome == scanner.FileUnchanged {
		report.status.Unchanged++
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
			report.status.Unchanged++
		default:
			if exists {
				report.status.Updated++
			} else {
				report.status.Imported++
			}
		}
	}
	episodes, err := s.Queries.GetShowFileEpisodes(ctx, id)
	if err != nil {
		return err
	}
	for _, episode := range episodes {
		report.scan.seasons[episode.SeasonID] = true
		report.scan.episodes[episode.ID] = true
	}
	return nil
}

func (s *Scanner) persistFile(ctx context.Context, local localEpisodeFile, file scanner.ScanFile, baseline database.GetShowScanIndexRow, inspection *scanner.FileInspection, info *ffprobe.FfprobeResult) (int64, error) {
	var id int64
	err := s.tx.Run(ctx, func(q *database.Queries) error {
		current, err := q.GetShowFileByPath(ctx, file.Path)
		missing := errors.Is(err, sql.ErrNoRows)
		if err != nil && !missing {
			return err
		}
		// The catalog row moved or was replaced since discovery; leave it to the
		// next scan rather than writing against a stale identity.
		if current.ID != baseline.ID || current.FilePath != baseline.FilePath {
			return errStaleCatalogRow
		}
		id = current.ID
		if info != nil {
			show, err := q.UpsertLocalShow(ctx, database.UpsertLocalShowParams{DirectoryPath: local.showPath, LocalName: local.title, PremiereYear: helpers.NullInt64(int64(local.year)), Name: local.title})
			if err != nil {
				return err
			}
			season, err := q.UpsertLocalShowSeason(ctx, database.UpsertLocalShowSeasonParams{ShowID: show.ID, SeasonNumber: int64(local.season), Name: fmt.Sprintf("Season %d", local.season)})
			if err != nil {
				return err
			}
			if current.ID != 0 && current.SeasonID != season.ID {
				return errors.New("file season ownership changed")
			}
			var duration sql.NullFloat64
			seconds, parseErr := strconv.ParseFloat(info.Format.Duration, 64)
			validDuration := parseErr == nil && seconds > 0 && !math.IsInf(seconds, 0) && !math.IsNaN(seconds)
			if validDuration {
				duration = helpers.NullFloat64(seconds)
			}
			stored, err := q.UpsertShowFile(ctx, database.UpsertShowFileParams{SeasonID: season.ID, FilePath: file.Path, FileName: filepath.Base(file.Path), Size: inspection.Fingerprint.Size, Container: file.Ext, MimeType: helpers.VideoMimeTypes[file.Ext], Duration: duration})
			if err != nil {
				return err
			}
			id = stored.ID
			count, err := processStreams(ctx, q, id, info.Streams)
			if err != nil {
				return err
			}
			if count == 0 {
				return errors.New("no accepted video stream")
			}
			err = processChapters(ctx, q, id, info.Chapters)
			if err != nil {
				return err
			}
			err = q.DeleteShowFileLinks(ctx, id)
			if err != nil {
				return err
			}
			for order, number := range local.episodes {
				ep, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{SeasonID: season.ID, EpisodeNumber: int64(number), Name: fmt.Sprintf("Episode %d", number)})
				if err != nil {
					return err
				}
				err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{EpisodeID: ep.ID, FileID: id, SeasonID: season.ID, EpisodeOrder: int64(order)})
				if err != nil {
					return err
				}
				err = q.MarkShowEpisodeRetry(ctx, ep.ID)
				if err != nil {
					return err
				}
			}
			err = q.MarkShowRetry(ctx, show.ID)
			if err != nil {
				return err
			}
			err = q.MarkShowSeasonRetry(ctx, season.ID)
			if err != nil {
				return err
			}
			err = q.PruneShowEpisodes(ctx)
			if err != nil {
				return err
			}
		}
		f := inspection.Fingerprint
		err = q.UpsertShowFingerprint(ctx, database.UpsertShowFingerprintParams{FileID: id, MtimeNs: f.MtimeNS, CtimeNs: f.CtimeNS, Device: f.Device, Inode: f.Inode, Sha256: f.SHA256[:]})
		if err != nil {
			return err
		}
		return inspection.Validate(ctx)
	}, nil)
	if errors.Is(err, errStaleCatalogRow) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Scanner) cleanup(ctx context.Context, r *scanner.Reconciliation, report *scanReport) error {
	err := r.ValidateRoot(ctx)
	if err != nil {
		return err
	}
	deleted, err := r.DeleteUnseen(ctx, func(file scanner.CatalogFile) (bool, error) {
		return s.deleteMissing(ctx, r, file)
	})
	report.status.Deleted += deleted
	if err != nil {
		return err
	}
	return ctx.Err()
}

func (s *Scanner) deleteMissing(ctx context.Context, r *scanner.Reconciliation, file scanner.CatalogFile) (bool, error) {
	return r.DeleteConfirmed(ctx, s.tx, file, func(q *database.Queries) (bool, error) {
		count, err := q.DeleteMissingShowFile(ctx, database.DeleteMissingShowFileParams{ID: file.ID, FilePath: file.Path})
		if err != nil || count == 0 {
			return false, err
		}
		// Cascades leave the catalog rows behind, so prune bottom-up: episodes
		// without files, then empty seasons, then empty shows.
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
		return true, nil
	}, nil)
}
