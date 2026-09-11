package music

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

const (
	progressLogInterval = 10 * time.Second
	// Music never retries a deferred file within a run; the next scan does.
	reasonDeferred = "The file is still changing or has not been quiet for 60 seconds. It is retried on the next scan."
)

// Start launches a scan asynchronously when configured and no music scan is running.
func (s *Scanner) Start() StartResult {
	directory := s.currentMusicDirectory()
	if !directory.Valid || directory.String == "" {
		return StartResult{Status: StartNotConfigured}
	}

	started := s.guard.TryBegin()
	if !started {
		return StartResult{Directory: directory.String, Status: StartAlreadyRunning}
	}

	// The run is published before the goroutine starts so a status poll right
	// after the request already sees it.
	s.beginReport()

	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		defer s.guard.Finish()
		s.runMusicScan(directory.String)
	}()
	return StartResult{Directory: directory.String, Status: StartStarted}
}

// runMusicScan expects beginReport to have published the run it continues.
func (s *Scanner) runMusicScan(directory string) {
	report := newScanReport(s.Status())
	ctx := s.scanContext
	stopProgressLog := scanner.StartProgressLog(progressLogInterval, func() {
		status := s.Status()
		s.logger.Info("music scan progress", "run", status.RunID, "phase", status.Phase, "processed", status.Processed, "total", status.Total, "enriched", status.Enriched)
	})
	defer func() {
		stopProgressLog()
		contextErr := ctx.Err()
		if contextErr != nil {
			s.logger.Info("music library scan interrupted", "phase", report.status.Phase, "error", contextErr)
		}
		report.Finish(&report.status.Progress, contextErr != nil)
		s.publish(report)
		now := *report.status.FinishedAt
		s.logger.Info("music scan finished", "run", report.status.RunID, "state", report.status.State, "elapsed", now.Sub(*report.status.StartedAt), "processed", report.status.Processed, "total", report.status.Total, "imported", report.status.Imported, "updated", report.status.Updated, "unchanged", report.status.Unchanged, "failed", report.status.Failed, "deferred", report.status.Deferred, "deleted", report.status.Deleted, "matched", report.status.Enriched, "enrichment_failed", report.status.EnrichmentFailed, "unmatched", report.status.EnrichmentUnmatched)
	}()
	// fail marks a fatal error; a cancellation is reported once by the deferred
	// finish instead.
	fail := func(err error, reason string) {
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		s.logger.Error("music scan failed", "phase", report.status.Phase, "error", err)
		report.status.State = scanner.StateFailed
		report.Issue("", report.status.Phase, reason)
	}
	s.logger.Info("music scan phase", "run", report.status.RunID, "phase", scanner.PhaseLocal, "directory", directory)
	scanIndex, files, err := s.loadMusicScanIndex(ctx)
	if err != nil {
		fail(err, "Unable to read the music catalog.")
		return
	}
	scan := newMusicScanContext(scanIndex)
	report.scan = scan
	reconciliation, err := scanner.NewReconciliation(ctx, directory, files)
	if err != nil {
		fail(err, "The library directory is unavailable.")
		return
	}
	batch := make([]scanner.ScanFile, 0, scanner.BatchSize)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		s.processMusicBatch(ctx, scan, report, batch)
		batch = batch[:0]
	}

	err = scanner.WalkMediaLibraryContext(
		ctx,
		directory,
		helpers.ValidAudioExtensions,
		func(err error) {
			s.logger.Warn("music discovery failed", "error", err)
			var pathError *os.PathError
			filename := ""
			isPathError := errors.As(err, &pathError)
			if isPathError {
				filename = pathError.Path
			}
			report.Issue(filename, scanner.PhaseLocal, scanner.ReasonDiscoveryEntry)
		},
		func(file scanner.ScanFile) error {
			reconciliation.MarkSeen(file.Path)
			batch = append(batch, file)
			report.status.Total++

			if len(batch) >= scanner.BatchSize {
				flushBatch()
			}

			return nil
		},
	)
	if err != nil {
		fail(err, "Library discovery was interrupted; missing files were not removed.")
		return
	}

	flushBatch()
	contextErr := ctx.Err()
	if contextErr != nil {
		return
	}

	s.phase(report, scanner.PhaseCleanup)
	report.status.Deleted, err = s.cleanupMissingMusic(ctx, scan, reconciliation)
	if err != nil {
		fail(err, "Cleanup stopped because the library could not be safely checked.")
		return
	}

	s.phase(report, scanner.PhaseEnrichment)
	err = s.retrySpotify(ctx, scan, report)
	if err != nil {
		fail(err, "Spotify retries stopped after an error; remaining artists and albums are retried on the next scan.")
	}
}

// processMusicBatch records one local outcome per file on the report. A file
// interrupted by cancellation is left unrecorded.
func (s *Scanner) processMusicBatch(ctx context.Context, scan *musicScanContext, report *scanReport, files []scanner.ScanFile) {
	for _, file := range files {
		report.Activate(file.Path)
		s.publish(report)
		outcome, existed, err := s.processFile(ctx, scan, file)
		report.Deactivate(file.Path)
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		report.status.Processed++
		var deferred *scanner.FileDeferral
		isDeferred := errors.As(err, &deferred)
		switch {
		case isDeferred:
			report.status.Deferred++
			report.Issue(file.Path, scanner.PhaseLocal, reasonDeferred)
			s.logger.Debug("deferred track", "path", file.Path, "reason", deferred.Reason, "eligible_at", deferred.EligibleAt)
		case err != nil:
			report.status.Failed++
			report.Issue(file.Path, scanner.PhaseLocal, "Unable to inspect, probe, or save this track. Any previous record was preserved.")
			s.logger.Warn("failed to process track", "path", file.Path, "error", err)
		case outcome == scanner.FileDeferred:
			report.status.Deferred++
			report.Issue(file.Path, scanner.PhaseLocal, reasonDeferred)
		case outcome == scanner.FileUnchanged, outcome == scanner.FileFingerprintOnly:
			report.status.Unchanged++
		case existed:
			report.status.Updated++
		default:
			report.status.Imported++
		}
		s.publish(report)
	}
}

// processFile returns the file outcome and whether the cleaned path had a
// stored fingerprint baseline, which decides imported versus updated.
func (s *Scanner) processFile(ctx context.Context, scan *musicScanContext, file scanner.ScanFile) (outcome scanner.FileOutcome, existed bool, err error) {
	file.Path = filepath.Clean(file.Path)
	var previous *scanner.FileFingerprint
	baseline, existed := scan.trackIndex[file.Path]
	if existed {
		previous = &baseline
	}
	inspection, err := scanner.InspectFile(ctx, file.Path, previous, s.now)
	if err != nil {
		return outcome, existed, err
	}
	defer func() { err = errors.Join(err, inspection.Close()) }()
	outcome = inspection.Outcome
	if outcome == scanner.FileDeferred {
		s.logger.Debug("deferred track", "path", file.Path, "reason", inspection.Reason, "eligible_at", inspection.EligibleAt)
		return outcome, existed, nil
	}
	if outcome == scanner.FileUnchanged {
		return outcome, existed, nil
	}
	if outcome == scanner.FileFingerprintOnly {
		err = s.persistFingerprint(ctx, scan, file.Path, inspection)
		return outcome, existed, err
	}
	file.Size = inspection.Fingerprint.Size
	resolved, resolveErr := s.resolveTrackFile(ctx, scan, file)
	err = inspection.Validate(ctx)
	if err != nil {
		return outcome, existed, err
	}
	if resolveErr != nil {
		return outcome, existed, resolveErr
	}
	resolved.inspection = inspection
	_, err = s.persistResolvedTrack(ctx, scan, resolved)
	return outcome, existed, err
}

func (s *Scanner) loadMusicScanIndex(ctx context.Context) (map[string]scanner.FileFingerprint, []scanner.CatalogFile, error) {
	rows, err := s.queries.ListMusicTrackScanIndex(ctx)
	if err != nil {
		return nil, nil, err
	}
	index := make(map[string]scanner.FileFingerprint, len(rows))
	files := make([]scanner.CatalogFile, 0, len(rows))
	for _, row := range rows {
		files = append(files, scanner.CatalogFile{ID: row.ID, Path: row.FilePath})
		if !row.MtimeNs.Valid {
			continue
		}
		fingerprint := scanner.StoredFingerprint(row.Size, row.MtimeNs, row.CtimeNs, row.Device, row.Inode)
		fingerprint.SHA256 = [32]byte(row.Sha256)
		index[filepath.Clean(row.FilePath)] = fingerprint
	}
	return index, files, nil
}
