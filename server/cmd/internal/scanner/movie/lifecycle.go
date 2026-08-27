package movie

import (
	"context"
	"errors"
	"fmt"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) Start() StartResult {
	directory := s.currentMoviesDirectory()
	result := StartResult{Directory: directory.String}
	if !directory.Valid || directory.String == "" {
		result.Status = StartNotConfigured
		return result
	}

	if !s.guard.TryBegin() {
		result.Status = StartAlreadyRunning
		return result
	}

	// Add/Done are paired here so runMovieScan stays callable on its own.
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		s.runMovieScan()
	}()
	result.Status = StartStarted
	return result
}

func (s *Scanner) runMovieScan() {
	defer s.guard.Finish()

	directory := s.currentMoviesDirectory()
	if !directory.Valid || directory.String == "" {
		s.logger.Info("skipping movie library scan: movies directory is not configured")
		return
	}

	s.logger.Info(fmt.Sprintf("scanning movies directory: %s", directory.String))

	ctx := s.scanContext
	errorCount := 0
	moviesScanned := 0
	moviesSkipped := 0
	startTime := time.Now()

	scanIndex, err := s.loadMovieScanIndex(ctx)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to load movie scan index: %s", err.Error()))
		return
	}
	scan := newMovieScanContext(scanIndex)

	batch := make([]scanner.ScanFile, 0, scanner.BatchSize)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		scanned, skipped, batchErrors := s.processMoviesBatch(ctx, scan, batch)
		moviesScanned += scanned
		moviesSkipped += skipped
		errorCount += batchErrors
		batch = batch[:0]
	}

	err = scanner.WalkMediaLibraryContext(
		ctx,
		directory.String,
		helpers.ValidVideoExtensions,
		func(err error) {
			s.logger.Error(err.Error())
			errorCount++
		},
		func(file scanner.ScanFile) error {
			if scan.movieUnchanged(file.Path, file.Size) {
				moviesSkipped++
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
		if errors.Is(err, context.Canceled) {
			s.logger.Info("movie library scan canceled")
			return
		}
		s.logger.Error(fmt.Sprintf("unexpected error walking movies directory: %s", err.Error()))
		return
	}

	flushBatch()

	s.logger.Info(fmt.Sprintf("movies scanner completed: %d scanned, %d skipped, %d errors in %s",
		moviesScanned, moviesSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (s *Scanner) processMoviesBatch(ctx context.Context, scan *movieScanContext, files []scanner.ScanFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		if ctx.Err() != nil {
			return scanned, skipped, errCount
		}

		if scan.movieUnchanged(file.Path, file.Size) {
			skipped++
			continue
		}

		resolved, err := s.resolveMovieFile(ctx, file)
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to process %s: %s", file.Path, err.Error()))
			errCount++
			continue
		}

		err = s.persistResolvedMovie(ctx, scan, resolved)
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to process %s: %s", file.Path, err.Error()))
			errCount++
			continue
		}

		scanned++
	}

	return scanned, skipped, errCount
}

func (s *Scanner) loadMovieScanIndex(ctx context.Context) (map[string]int64, error) {
	rows, err := s.queries.GetMovieScanIndex(ctx)
	if err != nil {
		return nil, err
	}

	return scanner.BuildScanIndex(rows, func(row database.GetMovieScanIndexRow) (string, int64) {
		return row.FilePath, row.Size
	}), nil
}
