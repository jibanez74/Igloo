package music

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/itunes"
	"igloo/cmd/internal/scanner"
)

const (
	reasonFailed         = "Unable to inspect, probe, or save this track. Any previous record was preserved."
	reasonSpotifyStopped = "Spotify lookups stopped after provider failures. Pending artists, albums and tracks will retry on a later scan."
	reasonAppleStopped   = "Apple Music lookups stopped after provider failures. Pending albums will retry on a later scan."
)

// Start launches a scan asynchronously when configured and no music scan is running.
func (s *Scanner) Start() scanner.StartResult {
	return s.launcher.Start(s.currentMusicDirectory, nil, s.beginReport, s.runMusicScan)
}

// StartChanges applies what the library watcher saw change.
func (s *Scanner) StartChanges(changes scanner.Changes) scanner.StartResult {
	return s.launcher.Start(s.currentMusicDirectory, &changes, s.beginReport, s.runMusicScan)
}

// runMusicScan expects beginReport to have published the run it continues.
// changes is nil for a library scan.
func (s *Scanner) runMusicScan(directory string, changes *scanner.Changes) {
	report := newScanReport(s.Status())
	ctx := s.scanContext
	stopProgressLog := scanner.StartProgressLog(scanner.ProgressLogInterval, func() {
		status := s.Status()
		s.logger.Info("music scan progress", "run", status.RunID, "phase", status.Phase, "processed", status.Processed, "total", status.Total, "enriched", status.Enriched)
	})
	defer func() {
		stopProgressLog()
		contextErr := ctx.Err()
		if contextErr != nil {
			// Warn, not Info: an interrupted run leaves the tail of the job
			// list undispatched, so whole albums can be missing from the
			// catalog with no other sign. Warn also rides the logger's severe-
			// record flush, which is what gets this line onto disk when the
			// cancellation came from shutdown.
			s.logger.Warn("music library scan interrupted", "phase", report.status.Phase, "processed", report.status.Processed, "total", report.status.Total, "error", contextErr)
		}
		report.Finish(&report.status.Progress, contextErr != nil)
		s.outstanding.Keep(&report.Report)
		s.publish(report)
		now := *report.status.FinishedAt
		s.logger.Info("music scan finished", "run", report.status.RunID, "state", report.status.State, "elapsed", now.Sub(*report.status.StartedAt), "processed", report.status.Processed, "total", report.status.Total, "imported", report.status.Imported, "updated", report.status.Updated, "unchanged", report.status.Unchanged, "failed", report.status.Failed, "deferred", report.status.Deferred, "deleted", report.status.Deleted, "matched", report.status.Enriched, "enrichment_failed", report.status.EnrichmentFailed, "unmatched", report.status.EnrichmentUnmatched, "tracks_linked", report.tracksLinked())
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
	paths := changes.Resolve(directory)
	s.outstanding.Carry(&report.Report, changes, paths)
	scanIndex, files, err := s.loadMusicScanIndex(ctx, changes, paths)
	if err != nil {
		fail(err, "Unable to read the music catalog.")
		return
	}
	scan := newMusicScanContext(scanIndex)
	scan.changes = changes
	scan.appleEnabled = s.apple != nil && s.appleLookupsEnabled()
	report.scan = scan
	reconciliation, err := scanner.NewReconciliation(ctx, directory, files)
	if err != nil {
		fail(err, scanner.ReasonLibraryUnavailable)
		return
	}
	batch := make([]scanner.ScanFile, 0, scanner.BatchSize)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		s.applyMoves(ctx, scan, report, reconciliation, batch)
		s.processMusicBatch(ctx, scan, report, directory, batch)
		batch = batch[:0]
	}

	onError := func(path string, err error) {
		s.logger.Warn("music discovery failed", "path", path, "error", err)
		// Music has no discovery phase of its own, but a walk error is one.
		report.Issue(path, scanner.PhaseDiscovery, scanner.ReasonDiscoveryEntry)
	}
	onFile := func(file scanner.ScanFile) error {
		reconciliation.MarkSeen(file.Path)
		batch = append(batch, file)
		report.status.Total++

		if len(batch) >= scanner.BatchSize {
			flushBatch()
		}

		return nil
	}
	err = scanner.WalkRun(ctx, directory, changes, paths, helpers.ValidAudioExtensions, onError, onFile, nil)
	if err != nil {
		fail(err, scanner.ReasonDiscoveryInterrupted)
		return
	}
	s.outstanding.Unread(&report.Report, changes)

	flushBatch()
	contextErr := ctx.Err()
	if contextErr != nil {
		return
	}

	s.phase(report, scanner.PhaseCleanup)
	kept, err := reconciliation.Guard(ctx, s.tx, changes, s.mountPoints)
	if err != nil {
		fail(err, scanner.ReasonCleanupUnsafe)
		return
	}
	if kept.Files > 0 {
		s.logger.Warn("music cleanup kept files that looked unmounted", "directory", directory, "files", kept.Files, "mount_points", kept.MountPoints)
		report.Kept(kept)
	}
	report.status.Deleted, err = s.cleanupMissingMusic(ctx, scan, reconciliation)
	if err != nil {
		fail(err, scanner.ReasonCleanupUnsafe)
		return
	}

	if changes.Scoped() {
		scan.scope, err = s.touchedScope(ctx, scan.touched)
		if err != nil {
			fail(err, "Unable to read the touched albums and artists.")
			return
		}
		// Merges of the local phase are already in the scope.
		scan.merged = false
	}

	s.phase(report, scanner.PhaseEnrichment)
	err = s.enrichFromApple(ctx, scan, report)
	if err != nil {
		fail(err, "Apple Music lookups stopped after an error; remaining albums are retried on the next scan.")
		return
	}
	err = s.retrySpotify(ctx, scan, report)
	if err != nil {
		fail(err, "Spotify retries stopped after an error; remaining artists and albums are retried on the next scan.")
	}
}

// processMusicBatch records one local outcome per file on the report. A file
// interrupted by cancellation is left unrecorded.
func (s *Scanner) processMusicBatch(ctx context.Context, scan *musicScanContext, report *scanReport, root string, files []scanner.ScanFile) {
	deferredReason := scan.changes.DeferredReason(scanner.ReasonFileDeferredNextScan)
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
			report.Issue(file.Path, scanner.PhaseLocal, deferredReason)
			s.logger.Debug("deferred track", "path", file.Path, "reason", deferred.Reason)
			// The watcher re-reads the file to learn when it is quiet.
			scan.changes.Deferred(root, file.Path, time.Time{})
		case err != nil:
			report.status.Failed++
			report.Issue(file.Path, scanner.PhaseLocal, reasonFailed)
			s.logger.Warn("failed to process track", "path", file.Path, "error", err)
		case outcome == scanner.FileDeferred:
			report.status.Deferred++
			report.Issue(file.Path, scanner.PhaseLocal, deferredReason)
			scan.changes.Deferred(root, file.Path, time.Time{})
		case outcome == scanner.FileUnchanged, outcome == scanner.FileFingerprintOnly:
			report.status.Unchanged++
			scan.touch(file.Path)
		case existed:
			report.status.Updated++
			scan.touch(file.Path)
		default:
			report.status.Imported++
			scan.touch(file.Path)
		}
		s.noteSpotifyStop(scan, report)
		s.publish(report)
	}
}

// processFile returns the file outcome and whether the cleaned path was in
// the catalog when the scan started, which decides imported versus updated.
func (s *Scanner) processFile(ctx context.Context, scan *musicScanContext, file scanner.ScanFile) (outcome scanner.FileOutcome, existed bool, err error) {
	file.Path = filepath.Clean(file.Path)
	var previous *scanner.FileFingerprint
	baseline, existed := scan.trackIndex[file.Path]
	if existed {
		previous = &baseline
	}
	inspection, err := scanner.InspectFile(ctx, file.Path, previous, s.now, scan.changes.QuietPeriod())
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
	catalog := s.readCatalogIDs(ctx, file, inspection)
	resolved, resolveErr := s.resolveTrackFile(ctx, scan, file, catalog)
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

// readCatalogIDs reads the Apple ids an iTunes purchase carries, whether or
// not Apple lookups are on: an unchanged file is never read again. A file
// whose metadata cannot be read has no ids.
func (s *Scanner) readCatalogIDs(ctx context.Context, file scanner.ScanFile, inspection *scanner.FileInspection) itunes.CatalogIDs {
	if file.Ext != "m4a" {
		return itunes.CatalogIDs{}
	}
	ids, err := itunes.ReadCatalogIDs(ctx, inspection, inspection.Fingerprint.Size)
	if err != nil {
		s.logger.Debug("unreadable iTunes catalog ids", "path", file.Path, "error", err)
		return itunes.CatalogIDs{}
	}
	return ids
}

// loadMusicScanIndex reads the catalog tracks the run covers: all of them,
// or for a run of changes only those at or beneath paths.
func (s *Scanner) loadMusicScanIndex(ctx context.Context, changes *scanner.Changes, paths []string) (map[string]scanner.FileFingerprint, []scanner.CatalogFile, error) {
	var rows []database.ListMusicTrackScanIndexRow
	var err error
	if changes.Scoped() {
		rows, err = scanner.LoadUnder(ctx, paths, func(ctx context.Context, path, lower, upper string) ([]database.ListMusicTrackScanIndexUnderRow, error) {
			return s.queries.ListMusicTrackScanIndexUnder(ctx, database.ListMusicTrackScanIndexUnderParams{Path: path, Lower: lower, Upper: upper})
		}, func(row database.ListMusicTrackScanIndexUnderRow) database.ListMusicTrackScanIndexRow {
			return database.ListMusicTrackScanIndexRow(row)
		})
	} else {
		rows, err = s.queries.ListMusicTrackScanIndex(ctx)
	}
	if err != nil {
		return nil, nil, err
	}
	index, files := musicScanIndex(rows)
	return index, files, nil
}

func musicScanIndex(rows []database.ListMusicTrackScanIndexRow) (map[string]scanner.FileFingerprint, []scanner.CatalogFile) {
	index := make(map[string]scanner.FileFingerprint, len(rows))
	files := make([]scanner.CatalogFile, 0, len(rows))
	for _, row := range rows {
		// A catalog row without a baseline still exists, so its file counts as
		// updated rather than imported; the zero fingerprint never matches, which
		// forces the reprobe that records a real one.
		fingerprint := scanner.StoredFingerprint(row.Size, row.MtimeNs, row.CtimeNs, row.Device, row.Inode)
		if len(row.Sha256) == len(fingerprint.SHA256) {
			fingerprint.SHA256 = [32]byte(row.Sha256)
		}
		files = append(files, scanner.CatalogFile{ID: row.ID, Path: row.FilePath, Fingerprint: fingerprint})
		index[filepath.Clean(row.FilePath)] = fingerprint
	}
	return index, files
}
