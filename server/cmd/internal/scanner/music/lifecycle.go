package music

import (
	"context"
	"fmt"
	"time"

	"igloo/cmd/internal/database"
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
	scanIndex, err := s.loadMusicScanIndex(ctx)
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
			unchanged := scan.trackUnchanged(file.Path, file.Size)
			if unchanged {
				tracksSkipped++
				return nil
			}

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
	s.logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s; Spotify: %d matched, %d failed, %d unmatched",
		tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime)), scan.enrichmentCounts[musicSpotifyStatusMatched], scan.enrichmentCounts[musicSpotifyStatusFailed], scan.enrichmentCounts[musicSpotifyStatusUnmatched]))
}

func (s *Scanner) processMusicBatch(ctx context.Context, scan *musicScanContext, files []scanner.ScanFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		contextErr := ctx.Err()
		if contextErr != nil {
			return scanned, skipped, errCount
		}

		unchanged := scan.trackUnchanged(file.Path, file.Size)
		if unchanged {
			skipped++
			continue
		}

		resolved, err := s.resolveTrackFile(ctx, scan, file)
		if err != nil {
			contextErr = ctx.Err()
			if contextErr != nil {
				return scanned, skipped, errCount
			}
			s.logger.Warn("failed to resolve music track", "path", file.Path, "error", err)
			errCount++
			continue
		}

		_, err = s.persistResolvedTrack(ctx, scan, resolved)
		if err != nil {
			contextErr = ctx.Err()
			if contextErr != nil {
				return scanned, skipped, errCount
			}
			s.logger.Warn("failed to persist music track", "path", file.Path, "error", err)
			errCount++
			continue
		}

		scanned++
	}

	return scanned, skipped, errCount
}

func (s *Scanner) loadMusicScanIndex(ctx context.Context) (map[string]int64, error) {
	rows, err := s.queries.ListMusicTrackScanIndex(ctx)
	if err != nil {
		return nil, err
	}

	return scanner.BuildScanIndex(rows, func(row database.ListMusicTrackScanIndexRow) (string, int64) {
		return row.FilePath, row.Size
	}), nil
}
