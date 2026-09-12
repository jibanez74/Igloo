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
	// scanWorkers bounds concurrent ffprobe runs.
	scanWorkers         = 2
	progressLogInterval = 10 * time.Second
	// TV defers a changing file for the run; the next scan retries it.
	reasonDeferred   = "The file is still changing or has not been quiet for 60 seconds. It is retried on the next scan."
	reasonRejected   = "The file could not be read or its name does not identify a season and episode. Existing records are preserved."
	reasonEnrichment = "TMDB metadata could not be applied. The local entry is usable and the next scan retries it."
	reasonUnmatched  = "TMDB returned no matching show. Rename the folder or wait for a later scan; repeated misses back off."
	reasonStopped    = "TMDB enrichment stopped after provider failures. Pending shows will retry on a later scan."
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
		s.Logger.Info("show scan finished", "run", report.status.RunID, "state", report.status.State, "elapsed", now.Sub(*report.status.StartedAt), "processed", report.status.Processed, "total", report.status.Total, "imported", report.status.Imported, "updated", report.status.Updated, "unchanged", report.status.Unchanged, "failed", report.status.Failed, "deferred", report.status.Deferred, "deleted", report.status.Deleted, "episodes", report.status.Episodes, "enriched", report.status.Enriched, "enrichment_failed", report.status.EnrichmentFailed, "enrichment_unmatched", report.status.EnrichmentUnmatched, "pending_enrichment", report.status.PendingEnrichment)
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

	index, catalog, err := s.loadScanIndex(ctx)
	if err != nil {
		fail(err, "Unable to read the TV catalog.")
		return
	}
	scan := newShowScanContext(index)
	report.scan = scan

	reconciliation, err := scanner.NewReconciliation(ctx, root, catalog)
	if err != nil {
		fail(err, "The library directory is unavailable.")
		return
	}

	files := make([]scanner.ScanFile, 0)
	err = scanner.WalkMediaLibraryContext(ctx, root, helpers.ValidVideoExtensions, func(path string, err error) {
		s.Logger.Warn("show walk error", "path", path, "error", err)
		report.Issue(path, scanner.PhaseDiscovery, scanner.ReasonDiscoveryEntry)
	}, func(file scanner.ScanFile) error {
		file.Path = filepath.Clean(file.Path)
		reconciliation.MarkSeen(file.Path)
		if strings.HasPrefix(filepath.Base(file.Path), ".") {
			return nil
		}
		files = append(files, file)
		report.status.Total = len(files)
		if len(files)%scanner.DiscoveryPublishInterval == 0 {
			s.publish(report)
		}
		return nil
	}, func(path string) bool { return allowDirectory(root, path) })
	if err != nil {
		fail(err, "The library directory could not be read.")
		return
	}

	s.phase(report, scanner.PhaseLocal)
	s.processLocal(ctx, scan, report, root, files)
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

// loadScanIndex reads the catalog files with their fingerprint baselines and
// episode links in two queries, so unchanged files need no query at all.
func (s *Scanner) loadScanIndex(ctx context.Context) (map[string]showScanEntry, []scanner.CatalogFile, error) {
	rows, err := s.Queries.GetShowScanIndex(ctx)
	if err != nil {
		return nil, nil, err
	}
	links, err := s.Queries.GetShowScanEpisodeLinks(ctx)
	if err != nil {
		return nil, nil, err
	}
	episodes := make(map[int64][]int64, len(rows))
	for _, link := range links {
		episodes[link.FileID] = append(episodes[link.FileID], link.EpisodeID)
	}
	index := make(map[string]showScanEntry, len(rows))
	catalog := make([]scanner.CatalogFile, 0, len(rows))
	for _, row := range rows {
		entry := showScanEntry{ID: row.ID, SeasonID: row.SeasonID, FilePath: row.FilePath, HasFingerprint: row.MtimeNs.Valid, Episodes: episodes[row.ID]}
		if entry.HasFingerprint {
			entry.FileFingerprint = scanner.StoredFingerprint(row.Size, row.MtimeNs, row.CtimeNs, row.Device, row.Inode)
		}
		index[filepath.Clean(row.FilePath)] = entry
		catalog = append(catalog, scanner.CatalogFile{ID: row.ID, Path: row.FilePath})
	}
	return index, catalog, nil
}

type probeJob struct {
	file     scanner.ScanFile
	baseline showScanEntry
	exists   bool
}

// probeResult carries a worker's parse, inspection, and probe. info is set
// only when the file needs processing; the coordinator persists it.
type probeResult struct {
	job        probeJob
	local      localEpisodeFile
	inspection *scanner.FileInspection
	info       *ffprobe.FfprobeResult
	err        error
}

// prepareFile runs on a worker: it parses, inspects, and probes without
// touching the database, so a worker never waits behind the coordinator's
// transaction. Parsing comes first so a malformed name never opens the file.
func (s *Scanner) prepareFile(ctx context.Context, root string, job probeJob) probeResult {
	result := probeResult{job: job}
	result.local, result.err = parseFile(root, job.file.Path)
	if result.err != nil {
		return result
	}
	var previous *scanner.FileFingerprint
	if job.baseline.HasFingerprint {
		previous = &job.baseline.FileFingerprint
	}
	result.inspection, result.err = scanner.InspectFileMetadata(ctx, job.file.Path, previous, s.Now)
	if result.err != nil || result.inspection.Outcome != scanner.FileNeedsProcessing {
		return result
	}
	info, err := s.Ffprobe.GetMetadata(ctx, job.file.Path)
	if err != nil {
		result.err = err
		return result
	}
	if info == nil {
		result.err = errors.New("ffprobe returned no metadata")
		return result
	}
	result.err = result.inspection.Validate(ctx)
	if result.err == nil {
		result.info = info
	}
	return result
}

// processLocal probes files on the worker pool and persists each result on
// this goroutine, which stays the sole owner of the scan context and report.
func (s *Scanner) processLocal(ctx context.Context, scan *showScanContext, report *scanReport, root string, files []scanner.ScanFile) {
	jobs := make([]probeJob, 0, len(files))
	for _, file := range files {
		baseline, exists := scan.index[file.Path]
		jobs = append(jobs, probeJob{file: file, baseline: baseline, exists: exists})
	}
	scanner.RunWorkers(ctx, scanWorkers, jobs, func() bool { return true },
		func(ctx context.Context, job probeJob) probeResult { return s.prepareFile(ctx, root, job) },
		func(job probeJob) {
			report.Activate(job.file.Path)
			s.publish(report)
		},
		func(result probeResult) {
			report.Deactivate(result.job.file.Path)
			contextErr := ctx.Err()
			if contextErr == nil {
				if result.err == nil && result.info != nil {
					result.err = s.persistFile(ctx, scan, result)
				}
				canceledDuringPersist := result.err != nil && ctx.Err() != nil
				if !canceledDuringPersist {
					s.recordLocal(report, scan, result)
				}
			}
			if result.inspection != nil {
				closeErr := result.inspection.Close()
				if closeErr != nil {
					s.Logger.Warn("close show inspection", "path", result.job.file.Path, "error", closeErr)
				}
			}
			s.publish(report)
		})
}

// recordLocal classifies one finished file. A stale catalog row is left to
// the next scan and counts as nothing.
func (s *Scanner) recordLocal(report *scanReport, scan *showScanContext, result probeResult) {
	path := result.job.file.Path
	report.status.Processed++
	var deferral *scanner.FileDeferral
	deferred := errors.As(result.err, &deferral)
	if !deferred && result.err == nil && result.inspection.Outcome == scanner.FileDeferred {
		deferred = true
		deferral = &scanner.FileDeferral{Reason: result.inspection.Reason, EligibleAt: result.inspection.EligibleAt}
	}
	switch {
	case deferred:
		report.status.Deferred++
		report.Issue(path, scanner.PhaseLocal, reasonDeferred)
		s.Logger.Debug("deferred show file", "path", path, "reason", deferral.Reason)
	case errors.Is(result.err, errStaleCatalogRow):
	case result.err != nil:
		report.status.Failed++
		report.Issue(path, scanner.PhaseLocal, reasonRejected)
		s.Logger.Warn("rejected show file", "path", path, "error", result.err)
	case result.inspection.Outcome == scanner.FileUnchanged:
		report.status.Unchanged++
		scan.touch(result.job.baseline.SeasonID, result.job.baseline.Episodes)
	case result.job.exists:
		report.status.Updated++
	default:
		report.status.Imported++
	}
}

func (s *Scanner) persistFile(ctx context.Context, scan *showScanContext, result probeResult) error {
	local, file, baseline, inspection, info := result.local, result.job.file, result.job.baseline, result.inspection, result.info
	var entry showScanEntry
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
		id := stored.ID
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
		linked := make([]int64, 0, len(local.episodes))
		// A confirmed TMDB match survives a technical change: a new mtime says
		// nothing about which show, season, or episode the file is, and
		// re-queueing it would rewrite every descriptive row. Only entities
		// without an identity are queued.
		for order, number := range local.episodes {
			ep, err := q.UpsertLocalShowEpisode(ctx, database.UpsertLocalShowEpisodeParams{SeasonID: season.ID, EpisodeNumber: int64(number), Name: fmt.Sprintf("Episode %d", number)})
			if err != nil {
				return err
			}
			err = q.LinkShowEpisodeFile(ctx, database.LinkShowEpisodeFileParams{EpisodeID: ep.ID, FileID: id, SeasonID: season.ID, EpisodeOrder: int64(order)})
			if err != nil {
				return err
			}
			if !ep.TmdbID.Valid {
				err = q.MarkShowEpisodeRetry(ctx, ep.ID)
				if err != nil {
					return err
				}
			}
			linked = append(linked, ep.ID)
		}
		if !show.TmdbID.Valid {
			err = q.MarkShowRetry(ctx, show.ID)
			if err != nil {
				return err
			}
		}
		if !season.TmdbID.Valid {
			err = q.MarkShowSeasonRetry(ctx, season.ID)
			if err != nil {
				return err
			}
		}
		err = q.PruneShowSeasonEpisodes(ctx, season.ID)
		if err != nil {
			return err
		}
		f := inspection.Fingerprint
		err = q.UpsertShowFingerprint(ctx, database.UpsertShowFingerprintParams{FileID: id, MtimeNs: f.MtimeNS, CtimeNs: f.CtimeNS, Device: f.Device, Inode: f.Inode})
		if err != nil {
			return err
		}
		entry = showScanEntry{FileFingerprint: f, ID: id, SeasonID: season.ID, FilePath: file.Path, HasFingerprint: true, Episodes: linked}
		return inspection.Validate(ctx)
	}, nil)
	if err != nil {
		return err
	}
	scan.index[file.Path] = entry
	scan.touch(entry.SeasonID, entry.Episodes)
	return nil
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

// deleteMissing removes one catalog file and prunes only what it could have
// emptied: episodes of its season, the season, then the show.
func (s *Scanner) deleteMissing(ctx context.Context, r *scanner.Reconciliation, file scanner.CatalogFile) (bool, error) {
	return r.DeleteConfirmed(ctx, s.tx, file, func(q *database.Queries) (bool, error) {
		seasonID, err := q.DeleteMissingShowFile(ctx, database.DeleteMissingShowFileParams{ID: file.ID, FilePath: file.Path})
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		err = q.PruneShowSeasonEpisodes(ctx, seasonID)
		if err != nil {
			return false, err
		}
		showID, err := q.PruneShowSeason(ctx, seasonID)
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		err = q.PruneShow(ctx, showID)
		if err != nil {
			return false, err
		}
		return true, nil
	}, nil)
}
