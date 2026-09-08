package movie

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

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
		s.runMovieScan(directory.String)
	}()
	result.Status = StartStarted
	return result
}

func (s *Scanner) runMovieScan(directory string) {
	defer s.guard.Finish()

	s.logger.Info(fmt.Sprintf("scanning movies directory: %s", directory))

	ctx := s.scanContext
	errorCount := 0
	moviesScanned := 0
	moviesSkipped := 0
	startTime := time.Now()

	scanIndex, files, err := s.loadMovieScanIndex(ctx)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to load movie scan index: %s", err.Error()))
		return
	}
	scan := newMovieScanContext(scanIndex)
	reconciliation, err := scanner.NewReconciliation(directory, files)
	if err != nil {
		s.logger.Error("cannot reconcile movie library", "error", err)
		return
	}

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
		directory,
		helpers.ValidVideoExtensions,
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
		if errors.Is(err, context.Canceled) {
			s.logger.Info("movie library scan canceled")
			return
		}
		s.logger.Error(fmt.Sprintf("unexpected error walking movies directory: %s", err.Error()))
		return
	}

	flushBatch()
	contextErr := ctx.Err()
	if contextErr != nil {
		s.logger.Info("movie library scan interrupted")
		return
	}

	deleted, err := s.cleanupMissingMovie(ctx, scan, reconciliation)
	s.logger.Info("movie missing-file cleanup", "deleted", deleted)
	if err != nil {
		s.logger.Error("movie missing-file cleanup interrupted", "error", err)
		return
	}

	s.logger.Info(fmt.Sprintf("movies scanner completed: %d scanned, %d skipped, %d errors in %s; %d deferred",
		moviesScanned, moviesSkipped, errorCount, helpers.FormatDuration(time.Since(startTime)), scan.deferred))
}

func (s *Scanner) processMoviesBatch(ctx context.Context, scan *movieScanContext, files []scanner.ScanFile) (scanned, skipped, errCount int) {
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
			s.logger.Debug("deferred movie", "path", file.Path, "reason", deferred.Reason, "eligible_at", deferred.EligibleAt)
		} else if err != nil {
			s.logger.Warn("failed to process movie", "path", file.Path, "error", err)
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

func (s *Scanner) processFile(ctx context.Context, scan *movieScanContext, file scanner.ScanFile) (outcome scanner.FileOutcome, err error) {
	file.Path = filepath.Clean(file.Path)
	var previous *scanner.FileFingerprint
	baseline, exists := scan.movieIndex[file.Path]
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
		s.logger.Debug("deferred movie", "path", file.Path, "reason", inspection.Reason, "eligible_at", inspection.EligibleAt)
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
	resolved, resolveErr := s.resolveMovieFile(ctx, file)
	err = inspection.Validate(ctx)
	if err != nil {
		return outcome, err
	}
	if resolveErr != nil {
		return outcome, resolveErr
	}
	resolved.inspection = inspection
	err = s.persistResolvedMovie(ctx, scan, resolved)
	return outcome, err
}

func (s *Scanner) loadMovieScanIndex(ctx context.Context) (map[string]scanner.FileFingerprint, []scanner.CatalogFile, error) {
	rows, err := s.queries.GetMovieScanIndex(ctx)
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
