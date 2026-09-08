package music

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
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

	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		defer s.guard.Finish()
		s.runMusicScan(directory.String)
	}()
	return StartResult{Directory: directory.String, Status: StartStarted}
}

func (s *Scanner) runMusicScan(directory string) {
	s.logger.Info(fmt.Sprintf("scanning music directory: %s", directory))

	ctx := s.scanContext
	errorCount := 0
	tracksScanned := 0
	tracksSkipped := 0
	startTime := time.Now()
	batch := make([]scanner.ScanFile, 0, scanner.BatchSize)
	scanIndex, files, err := s.loadMusicScanIndex(ctx)
	if err != nil {
		contextErr := ctx.Err()
		if contextErr != nil {
			s.logger.Info("music library scan interrupted")
			return
		}
		s.logger.Error(fmt.Sprintf("failed to load music scan index: %s", err.Error()))
		return
	}
	scan := newMusicScanContext(scanIndex)
	reconciliation, err := scanner.NewReconciliation(directory, files)
	if err != nil {
		s.logger.Error("cannot reconcile music library", "error", err)
		return
	}
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		scanned, skipped, errors := s.processMusicBatch(ctx, scan, batch)
		tracksScanned += scanned
		tracksSkipped += skipped
		errorCount += errors
		batch = batch[:0]
	}

	err = scanner.WalkMediaLibraryContext(
		ctx,
		directory,
		helpers.ValidAudioExtensions,
		func(err error) {
			s.logger.Error(err.Error())
			errorCount++
		},
		func(file scanner.ScanFile) error {
			reconciliation.MarkSeen(file.Path)
			batch = append(batch, file)

			if len(batch) >= scanner.BatchSize {
				flushBatch()
			}

			return nil
		},
	)

	if err != nil {
		contextErr := ctx.Err()
		if contextErr != nil {
			s.logger.Info("music library scan interrupted")
			return
		}
		s.logger.Error(fmt.Sprintf("unexpected error walking music directory: %s", err.Error()))
		return
	}

	flushBatch()
	contextErr := ctx.Err()
	if contextErr != nil {
		s.logger.Info("music library scan interrupted")
		return
	}

	deleted, err := s.cleanupMissingMusic(ctx, scan, reconciliation)
	s.logger.Info("music missing-file cleanup", "deleted", deleted)
	if err != nil {
		contextErr = ctx.Err()
		if contextErr != nil {
			s.logger.Info("music library scan interrupted")
			return
		}
		s.logger.Error("music missing-file cleanup failed", "error", err)
		return
	}

	err = s.retrySpotify(ctx, scan)
	if err != nil {
		contextErr = ctx.Err()
		if contextErr != nil {
			s.logger.Info("music library scan interrupted")
		} else {
			s.logger.Error("music Spotify retry failed", "error", err)
		}
		return
	}
	s.logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s; %d deferred; Spotify: %d matched, %d failed, %d unmatched",
		tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime)), scan.deferred, scan.enrichmentCounts[musicSpotifyStatusMatched], scan.enrichmentCounts[musicSpotifyStatusFailed], scan.enrichmentCounts[musicSpotifyStatusUnmatched]))
}

func (s *Scanner) processMusicBatch(ctx context.Context, scan *musicScanContext, files []scanner.ScanFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		outcome, err := s.processFile(ctx, scan, file)
		contextErr := ctx.Err()
		if contextErr != nil {
			return scanned, skipped, errCount
		}
		var deferred *scanner.FileDeferral
		isDeferred := errors.As(err, &deferred)
		if isDeferred {
			scan.deferred++
			s.logger.Debug("deferred track", "path", file.Path, "reason", deferred.Reason, "eligible_at", deferred.EligibleAt)
		} else if err != nil {
			s.logger.Warn("failed to process track", "path", file.Path, "error", err)
			errCount++
		} else {
			switch outcome {
			case scanner.FileDeferred:
				scan.deferred++
			case scanner.FileUnchanged, scanner.FileFingerprintOnly:
				skipped++
			case scanner.FileNeedsProcessing:
				scanned++
			}
		}
	}
	return scanned, skipped, errCount
}

func (s *Scanner) processFile(ctx context.Context, scan *musicScanContext, file scanner.ScanFile) (outcome scanner.FileOutcome, err error) {
	file.Path = filepath.Clean(file.Path)
	var previous *scanner.FileFingerprint
	baseline, exists := scan.trackIndex[file.Path]
	if exists {
		previous = &baseline
	}
	inspection, err := scanner.InspectFile(ctx, file.Path, previous, s.now)
	if err != nil {
		return outcome, err
	}
	defer func() { err = errors.Join(err, inspection.Close()) }()
	outcome = inspection.Outcome
	if outcome == scanner.FileDeferred {
		s.logger.Debug("deferred track", "path", file.Path, "reason", inspection.Reason, "eligible_at", inspection.EligibleAt)
		return outcome, nil
	}
	if outcome == scanner.FileUnchanged {
		return outcome, nil
	}
	if outcome == scanner.FileFingerprintOnly {
		err = s.persistFingerprint(ctx, scan, file.Path, inspection)
		return outcome, err
	}
	file.Size = inspection.Fingerprint.Size
	resolved, resolveErr := s.resolveTrackFile(ctx, scan, file)
	err = inspection.Validate(ctx)
	if err != nil {
		return outcome, err
	}
	if resolveErr != nil {
		return outcome, resolveErr
	}
	resolved.inspection = inspection
	_, err = s.persistResolvedTrack(ctx, scan, resolved)
	return outcome, err
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
		index[filepath.Clean(row.FilePath)] = scanner.FileFingerprint{
			Size: row.Size, MtimeNS: row.MtimeNs.Int64, CtimeNS: row.CtimeNs.Int64,
			Device: row.Device.String, Inode: row.Inode.String, SHA256: [32]byte(row.Sha256),
		}
	}
	return index, files, nil
}
