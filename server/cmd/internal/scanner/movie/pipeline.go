package movie

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

const (
	// scanWorkers bounds concurrent ffprobe runs and TMDB lookups. Raising it
	// needs a TMDB rate limiter first.
	scanWorkers         = 2
	slowOperation       = 5 * time.Second
	progressLogInterval = 10 * time.Second
	deferredRetryWindow = 120 * time.Second
	maxDeferredRetries  = 2
)

type fileState uint8

const (
	fileUnprocessed fileState = iota
	fileDeferred
	fileFailed
	fileUnchanged
	fileImported
)

type localFile struct {
	file     scanner.ScanFile
	state    fileState
	eligible time.Time
	retries  int
}

type probeJob struct {
	index    int
	file     scanner.ScanFile
	baseline movieScanEntry
}

type probeResult struct {
	job        probeJob
	inspection *scanner.FileInspection
	resolved   *localMovie
	err        error
}

// logSlow reports operations that took at least slowOperation. Defer it with
// time.Now() so the start is captured when the call is deferred.
func (s *Scanner) logSlow(message string, started time.Time, args ...any) {
	elapsed := time.Since(started)
	if elapsed >= slowOperation {
		s.logger.Info(message, append(args, "elapsed", elapsed)...)
	}
}

func (s *Scanner) closeInspection(inspection *scanner.FileInspection, path string) {
	if inspection == nil {
		return
	}
	err := inspection.Close()
	if err != nil {
		s.logger.Warn("close movie inspection", "path", path, "error", err)
	}
}

// prepareFile runs on a worker: it inspects and probes without touching the
// database, so a worker never waits behind the coordinator's transaction.
func (s *Scanner) prepareFile(ctx context.Context, job probeJob) probeResult {
	defer s.logSlow("slow movie inspection/probe", time.Now(), "path", job.file.Path)
	result := probeResult{job: job}
	var previous *scanner.FileFingerprint
	if job.baseline.HasFingerprint {
		previous = &job.baseline.FileFingerprint
	}
	result.inspection, result.err = scanner.InspectFileMetadata(ctx, job.file.Path, previous, s.now)
	if result.err != nil || result.inspection.Outcome != scanner.FileNeedsProcessing {
		return result
	}
	job.file.Size = result.inspection.Fingerprint.Size
	result.resolved, result.err = s.resolveLocalMovie(ctx, job.file, job.baseline)
	validationErr := result.inspection.Validate(ctx)
	if validationErr != nil {
		result.err = validationErr
	}
	if result.err == nil {
		result.resolved.inspection = result.inspection
	}
	return result
}

// runMovieScan expects beginReport to have published the run it continues.
func (s *Scanner) runMovieScan(directory string) {
	report := newScanReport(s.Status())
	ctx := s.scanContext
	stopProgressLog := scanner.StartProgressLog(progressLogInterval, func() {
		status := s.Status()
		s.logger.Info("movie scan progress", "run", status.RunID, "phase", status.Phase, "processed", status.Processed, "total", status.Total, "enriched", status.Enriched)
	})
	defer func() {
		stopProgressLog()
		report.Finish(&report.status.Progress, ctx.Err() != nil)
		s.publish(report)
		now := *report.status.FinishedAt
		s.logger.Info("movie scan finished", "run", report.status.RunID, "state", report.status.State, "elapsed", now.Sub(*report.status.StartedAt), "processed", report.status.Processed, "total", report.status.Total, "imported", report.status.Imported, "updated", report.status.Updated, "unchanged", report.status.Unchanged, "failed", report.status.Failed, "deferred", report.status.Deferred, "deleted", report.status.Deleted, "enriched", report.status.Enriched, "pending", report.status.PendingEnrichment)
	}()
	fail := func(err error, reason string) {
		contextErr := ctx.Err()
		if contextErr != nil {
			s.logger.Info("movie scan interrupted", "phase", report.status.Phase, "error", contextErr)
			return
		}
		s.logger.Error("movie scan interrupted", "phase", report.status.Phase, "error", err)
		report.status.State = scanner.StateFailed
		report.Issue("", report.status.Phase, reason)
	}
	s.logger.Info("movie scan phase", "run", report.status.RunID, "phase", scanner.PhaseDiscovery, "directory", directory)
	index, catalog, err := s.loadMovieScanIndex(ctx)
	if err != nil {
		fail(err, "Unable to read the movie catalog.")
		return
	}
	scan := newMovieScanContext(index)
	report.scan = scan
	reconciliation, err := scanner.NewReconciliation(ctx, directory, catalog)
	if err != nil {
		fail(err, "The library directory is unavailable.")
		return
	}
	files := make([]localFile, 0)
	err = scanner.WalkMediaLibraryContext(ctx, directory, helpers.ValidVideoExtensions,
		func(path string, err error) {
			s.logger.Warn("movie discovery failed", "path", path, "error", err)
			report.Issue(path, scanner.PhaseDiscovery, scanner.ReasonDiscoveryEntry)
		},
		func(file scanner.ScanFile) error {
			file.Path = filepath.Clean(file.Path)
			reconciliation.MarkSeen(file.Path)
			files = append(files, localFile{file: file})
			report.status.Total = len(files)
			if len(files)%scanner.DiscoveryPublishInterval == 0 {
				s.publish(report)
			}
			return nil
		})
	if err != nil {
		fail(err, "Library discovery was interrupted; missing files were not removed.")
		return
	}
	s.phase(report, scanner.PhaseLocal)
	indexes := make([]int, len(files))
	for i := range files {
		indexes[i] = i
	}
	s.processLocal(ctx, scan, report, files, indexes, time.Time{})
	contextErr := ctx.Err()
	if contextErr != nil {
		return
	}
	s.retryDeferred(ctx, scan, report, files)
	contextErr = ctx.Err()
	if contextErr != nil {
		return
	}
	s.phase(report, scanner.PhaseCleanup)
	report.status.Deleted, err = s.cleanupMissingMovie(ctx, scan, reconciliation)
	if err != nil {
		fail(err, "Cleanup stopped because the library could not be safely checked.")
		return
	}
	s.phase(report, scanner.PhaseEnrichment)
	s.enrichMovies(ctx, scan, report, files)
}

// processLocal probes the files at indexes on the worker pool and persists
// each result on this goroutine. Dispatch stops once dispatchBefore passes.
func (s *Scanner) processLocal(ctx context.Context, scan *movieScanContext, report *scanReport, files []localFile, indexes []int, dispatchBefore time.Time) {
	jobs := make([]probeJob, 0, len(indexes))
	for _, index := range indexes {
		file := files[index].file
		jobs = append(jobs, probeJob{index: index, file: file, baseline: scan.movieIndex[file.Path]})
	}
	withinWindow := func() bool {
		return dispatchBefore.IsZero() || !s.now().After(dispatchBefore)
	}
	scanner.RunWorkers(ctx, scanWorkers, jobs, withinWindow, s.prepareFile,
		func(job probeJob) {
			if files[job.index].state == fileDeferred {
				files[job.index].retries++
			}
			report.Activate(job.file.Path)
			s.publish(report)
		},
		func(result probeResult) {
			report.Deactivate(result.job.file.Path)
			contextErr := ctx.Err()
			if contextErr == nil {
				if result.err == nil && result.resolved != nil {
					result.err = s.persistLocalMovie(ctx, scan, result.resolved)
				}
				canceledDuringPersist := result.err != nil && ctx.Err() != nil
				if !canceledDuringPersist {
					s.recordLocal(report, &files[result.job.index], result)
				}
			}
			s.closeInspection(result.inspection, result.job.file.Path)
			s.publish(report)
		})
}

func (s *Scanner) recordLocal(report *scanReport, file *localFile, result probeResult) {
	if file.state == fileUnprocessed {
		report.status.Processed++
	}
	if file.state == fileDeferred {
		report.status.Deferred--
	}
	report.DropIssue(file.file.Path, scanner.PhaseLocal)
	var deferred *scanner.FileDeferral
	isDeferred := errors.As(result.err, &deferred)
	switch {
	case isDeferred:
		file.state, file.eligible = fileDeferred, deferred.EligibleAt
	case result.err != nil:
		file.state = fileFailed
		report.status.Failed++
		report.Issue(file.file.Path, scanner.PhaseLocal, "Unable to inspect, probe, or save this movie. Any previous record was preserved.")
		s.logger.Warn("failed to process movie", "path", file.file.Path, "error", result.err)
	case result.inspection.Outcome == scanner.FileDeferred:
		file.state, file.eligible = fileDeferred, result.inspection.EligibleAt
	case result.inspection.Outcome == scanner.FileUnchanged:
		file.state = fileUnchanged
		report.status.Unchanged++
	default:
		file.state = fileImported
		if result.job.baseline.ID == 0 {
			report.status.Imported++
		} else {
			report.status.Updated++
		}
	}
	if file.state == fileDeferred {
		report.status.Deferred++
		if file.eligible.IsZero() {
			file.eligible = s.now().Add(scanner.FileQuietPeriod)
		}
		report.Issue(file.file.Path, scanner.PhaseLocal, "The file is still changing or has not been quiet for 60 seconds. It remains deferred until a later attempt.")
		s.logger.Info("deferred movie", "path", file.file.Path, "eligible_at", file.eligible)
	}
}

func (s *Scanner) retryDeferred(ctx context.Context, scan *movieScanContext, report *scanReport, files []localFile) {
	deadline := s.now().Add(deferredRetryWindow)
	for {
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		var earliest time.Time
		for _, file := range files {
			retryable := file.state == fileDeferred && file.retries < maxDeferredRetries && !file.eligible.After(deadline)
			if retryable && (earliest.IsZero() || file.eligible.Before(earliest)) {
				earliest = file.eligible
			}
		}
		if earliest.IsZero() || s.now().After(deadline) {
			return
		}
		delay := earliest.Sub(s.now())
		if delay > 0 {
			s.phase(report, scanner.PhaseRetryWait)
			err := s.waitForRetry(ctx, delay)
			if err != nil {
				return
			}
		}
		indexes := make([]int, 0)
		now := s.now()
		for i := range files {
			file := &files[i]
			retryable := file.state == fileDeferred && file.retries < maxDeferredRetries && !file.eligible.After(now)
			if retryable {
				indexes = append(indexes, i)
			}
		}
		s.phase(report, scanner.PhaseLocal)
		s.processLocal(ctx, scan, report, files, indexes, deadline)
	}
}

func waitForMovieRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
