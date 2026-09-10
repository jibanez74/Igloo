package movie

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

type localFile struct {
	file     scanner.ScanFile
	state    string
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
	resolved   *resolvedMovie
	err        error
}

func (s *Scanner) prepareFile(ctx context.Context, job probeJob) probeResult {
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		if elapsed >= 5*time.Second {
			s.logger.Info("slow movie inspection/probe", "path", job.file.Path, "elapsed", elapsed)
		}
	}()
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
	result.resolved, result.err = s.resolveLocalMovie(ctx, job.file)
	validationErr := result.inspection.Validate(ctx)
	if validationErr != nil {
		result.err = validationErr
	}
	if result.err == nil && result.resolved.observed.ID != job.baseline.ID {
		result.err = &scanner.FileDeferral{Reason: scanner.FileChanged}
	}
	if result.err == nil {
		result.resolved.inspection = result.inspection
	}
	return result
}

func (s *Scanner) runMovieScan(directory string) {
	defer s.guard.Finish()
	currentStatus := s.Status()
	if currentStatus.State != "running" {
		s.beginReport()
	}
	report := &scanReport{status: s.Status(), issues: make(map[string]Issue), active: make(map[string]bool)}
	ctx := s.scanContext
	var scan *movieScanContext
	pendingCounted := false
	done := make(chan struct{})
	var logging sync.WaitGroup
	logging.Add(1)
	go func() {
		defer logging.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				status := s.Status()
				s.logger.Info("movie scan progress", "run", status.RunID, "phase", status.Phase, "processed", status.Processed, "total", status.Total, "enriched", status.Enriched)
			}
		}
	}()
	defer func() {
		close(done)
		logging.Wait()
		if scan != nil && !pendingCounted {
			report.status.PendingEnrichment = 0
			for _, entry := range scan.movieIndex {
				if entry.PendingRetry || !entry.TmdbID.Valid {
					report.status.PendingEnrichment++
				}
			}
		}
		contextErr := ctx.Err()
		if contextErr != nil {
			report.status.State = "canceled"
		}
		if report.status.State == "running" {
			report.status.State = "completed"
			if len(report.issues) > 0 || report.status.PendingEnrichment > 0 {
				report.status.State = "completed-with-issues"
			}
		}
		report.active = make(map[string]bool)
		now := time.Now().UTC()
		report.status.FinishedAt = &now
		s.publish(report)
		s.logger.Info("movie scan finished", "run", report.status.RunID, "state", report.status.State, "elapsed", now.Sub(*report.status.StartedAt), "processed", report.status.Processed, "total", report.status.Total, "imported", report.status.Imported, "updated", report.status.Updated, "unchanged", report.status.Unchanged, "failed", report.status.Failed, "deferred", report.status.Deferred, "deleted", report.status.Deleted, "enriched", report.status.Enriched, "pending", report.status.PendingEnrichment)
	}()
	fail := func(err error, reason string) {
		contextErr := ctx.Err()
		if contextErr != nil {
			s.logger.Info("movie scan interrupted", "phase", report.status.Phase, "error", contextErr)
			return
		}
		s.logger.Error("movie scan interrupted", "phase", report.status.Phase, "error", err)
		report.status.State = "failed"
		report.issue("", report.status.Phase, reason)
	}
	s.logger.Info("movie scan phase", "run", report.status.RunID, "phase", "discovery", "directory", directory)
	index, catalog, err := s.loadMovieScanIndex(ctx)
	if err != nil {
		fail(err, "Unable to read the movie catalog.")
		return
	}
	scan = newMovieScanContext(index)
	for _, entry := range index {
		if entry.PendingRetry || !entry.TmdbID.Valid {
			report.status.PendingEnrichment++
		}
	}
	reconciliation, err := scanner.NewReconciliation(directory, catalog)
	if err != nil {
		fail(err, "The library directory is unavailable.")
		return
	}
	files := make([]localFile, 0)
	err = scanner.WalkMediaLibraryContext(ctx, directory, helpers.ValidVideoExtensions,
		func(err error) {
			s.logger.Warn("movie discovery failed", "error", err)
			var pathError *os.PathError
			filename := ""
			isPathError := errors.As(err, &pathError)
			if isPathError {
				filename = filepath.Base(pathError.Path)
			}
			report.issues[fmt.Sprintf("discovery:%d", len(report.issues))] = Issue{Filename: filename, Phase: "discovery", Reason: "A library entry could not be inspected. Existing records are preserved."}
		},
		func(file scanner.ScanFile) error {
			file.Path = filepath.Clean(file.Path)
			reconciliation.MarkSeen(file.Path)
			files = append(files, localFile{file: file})
			report.status.Total = len(files)
			s.publish(report)
			return nil
		})
	if err != nil {
		fail(err, "Library discovery was interrupted; missing files were not removed.")
		return
	}
	s.phase(report, "local")
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
	s.phase(report, "cleanup")
	report.status.Deleted, err = s.cleanupMissingMovie(ctx, scan, reconciliation)
	if err != nil {
		fail(err, "Cleanup stopped because the library could not be safely checked.")
		return
	}
	s.phase(report, "enrichment")
	s.enrichMovies(ctx, scan, report, files)
	contextErr = ctx.Err()
	if contextErr != nil {
		return
	}
	pending, err := s.queries.CountMovieTmdbRetries(ctx)
	if err != nil {
		fail(err, "Unable to read pending enrichment.")
		return
	}
	report.status.PendingEnrichment = int(pending)
	pendingCounted = true
}

func (s *Scanner) processLocal(ctx context.Context, scan *movieScanContext, report *scanReport, files []localFile, indexes []int, dispatchBefore time.Time) {
	jobs := make(chan probeJob)
	results := make(chan probeResult, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				results <- s.prepareFile(ctx, job)
			}
		}()
	}
	defer workers.Wait()
	next, active := 0, 0
	for next < len(indexes) || active > 0 {
		var dispatch chan probeJob
		var job probeJob
		contextErr := ctx.Err()
		withinWindow := dispatchBefore.IsZero() || !s.now().After(dispatchBefore)
		canDispatch := next < len(indexes) && active < 2 && contextErr == nil && withinWindow
		if canDispatch {
			index := indexes[next]
			file := files[index].file
			job = probeJob{index: index, file: file, baseline: scan.movieIndex[file.Path]}
			dispatch = jobs
		}
		if active == 0 && dispatch == nil {
			break
		}
		select {
		case dispatch <- job:
			if files[job.index].state == "deferred" {
				files[job.index].retries++
			}
			next++
			active++
			report.active[job.file.Path] = true
			s.publish(report)
		case result := <-results:
			active--
			delete(report.active, result.job.file.Path)
			contextErr := ctx.Err()
			if contextErr == nil {
				if result.err == nil && result.resolved != nil {
					result.err = s.persistResolvedMovie(ctx, scan, result.resolved)
				}
				contextErr := ctx.Err()
				if result.err == nil || contextErr == nil {
					s.recordLocal(report, &files[result.job.index], result)
					before := result.job.baseline
					after := scan.movieIndex[result.job.file.Path]
					wasPending := before.ID != 0 && (before.PendingRetry || !before.TmdbID.Valid)
					isPending := after.ID != 0 && (after.PendingRetry || !after.TmdbID.Valid)
					if !wasPending && isPending {
						report.status.PendingEnrichment++
					}
				}
			}
			if result.inspection != nil {
				err := result.inspection.Close()
				if err != nil {
					s.logger.Warn("close movie inspection", "path", result.job.file.Path, "error", err)
				}
			}
			s.publish(report)
		}
	}
	close(jobs)
}

func (s *Scanner) recordLocal(report *scanReport, file *localFile, result probeResult) {
	if file.state == "" {
		report.status.Processed++
	}
	if file.state == "deferred" {
		report.status.Deferred--
	}
	delete(report.issues, "local:"+file.file.Path)
	var deferred *scanner.FileDeferral
	isDeferred := errors.As(result.err, &deferred)
	switch {
	case isDeferred:
		file.state, file.eligible = "deferred", deferred.EligibleAt
	case result.err != nil:
		file.state = "failed"
		report.status.Failed++
		report.issue(file.file.Path, "local", "Unable to inspect, probe, or save this movie. Any previous record was preserved.")
		s.logger.Warn("failed to process movie", "path", file.file.Path, "error", result.err)
	case result.inspection.Outcome == scanner.FileDeferred:
		file.state, file.eligible = "deferred", result.inspection.EligibleAt
	case result.inspection.Outcome == scanner.FileUnchanged:
		file.state = "unchanged"
		report.status.Unchanged++
	default:
		file.state = "imported"
		if result.job.baseline.ID == 0 {
			report.status.Imported++
		} else {
			report.status.Updated++
		}
	}
	if file.state == "deferred" {
		report.status.Deferred++
		if file.eligible.IsZero() {
			file.eligible = s.now().Add(scanner.FileQuietPeriod)
		}
		report.issue(file.file.Path, "local", "The file is still changing or has not been quiet for 60 seconds. It remains deferred until a later attempt.")
		s.logger.Info("deferred movie", "path", file.file.Path, "eligible_at", file.eligible)
	}
}

func (s *Scanner) retryDeferred(ctx context.Context, scan *movieScanContext, report *scanReport, files []localFile) {
	deadline := s.now().Add(120 * time.Second)
	for {
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		var earliest time.Time
		for _, file := range files {
			if file.state == "deferred" && file.retries < 2 && !file.eligible.After(deadline) {
				if earliest.IsZero() || file.eligible.Before(earliest) {
					earliest = file.eligible
				}
			}
		}
		if earliest.IsZero() || s.now().After(deadline) {
			return
		}
		delay := earliest.Sub(s.now())
		if delay > 0 {
			s.phase(report, "retry-wait")
			err := s.waitForRetry(ctx, delay)
			if err != nil {
				return
			}
		}
		indexes := make([]int, 0)
		now := s.now()
		for i := range files {
			file := &files[i]
			if file.state == "deferred" && file.retries < 2 && !file.eligible.After(now) {
				indexes = append(indexes, i)
			}
		}
		s.phase(report, "local")
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
