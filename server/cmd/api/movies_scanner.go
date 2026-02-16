package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"path/filepath"
	"time"
)

// movieFile holds path, extension, and size collected during directory walk.
// Size is captured during walk to avoid blocking the transaction with file I/O.
type movieFile struct {
	path string
	ext  string
	size int64
}

// ScanMoviesLibrary walks through the configured movies directory, extracts metadata
// from video files using ffprobe and TMDB API, and stores movie information in the database.
func (app *Application) ScanMoviesLibrary() {
	if app.Wait != nil {
		app.Wait.Add(1)
		defer app.Wait.Done()
	}

	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		app.Logger.Error("movies directory not configured")
		return
	}

	ctx := context.Background()
	errorCount := 0
	moviesScanned := 0
	moviesSkipped := 0
	startTime := time.Now()

	// Initialize in-memory cache for artists and production companies
	cache := newMovieScannerCache()
	defer cache.Clear()

	// Batch buffer to collect movies before processing
	batch := make([]movieFile, 0, helpers.SCANNER_BATCH_SIZE)

	err := filepath.WalkDir(app.Settings.MoviesDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			app.Logger.Error(fmt.Sprintf("error walking directory: %s", err.Error()))
			errorCount++
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		ext := helpers.GetFileExtension(path)
		if !helpers.ValidVideoExtensions[ext] {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to get file info for %s: %s", path, err.Error()))
			errorCount++
			return nil
		}

		batch = append(batch, movieFile{path: path, ext: ext, size: info.Size()})

		// Process batch when full
		if len(batch) >= helpers.SCANNER_BATCH_SIZE {
			scanned, skipped, errors := app.processMoviesBatch(ctx, batch, cache)
			moviesScanned += scanned
			moviesSkipped += skipped
			errorCount += errors
			batch = batch[:0]
		}

		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("unexpected error walking movies directory: %s", err.Error()))
		return
	}

	// Process remaining movies in the final batch
	if len(batch) > 0 {
		scanned, skipped, errors := app.processMoviesBatch(ctx, batch, cache)
		moviesScanned += scanned
		moviesSkipped += skipped
		errorCount += errors
	}

	app.Logger.Info(fmt.Sprintf("movies scanner completed: %d scanned, %d skipped, %d errors in %s",
		moviesScanned, moviesSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

// processMoviesBatch processes a batch of movie files within a single transaction.
// Uses skip-on-error strategy: failed movies don't rollback successful ones.
// Holds ScannerDBMu so only one scanner (music or movie) writes to the DB at a time.
func (app *Application) processMoviesBatch(ctx context.Context, files []movieFile, cache *movieScannerCache) (scanned, skipped, errCount int) {
	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, len(files)
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	for _, file := range files {
		// Check if movie exists with same path and size (file unchanged)
		_, err = qtx.CheckMovieUnchanged(ctx, database.CheckMovieUnchangedParams{
			FilePath: file.path,
			Size:     file.size,
		})

		if err == nil {
			skipped++
			continue
		}

		// File is new or size changed - process it
		// Use savepoint to allow per-movie rollback on failure while continuing with other movies
		savepointName := fmt.Sprintf("sp_movie_%d", scanned+skipped+errCount)

		err = manageSavepoint(ctx, tx, savepointName, func() error {

			return app.processMovieFile(ctx, qtx, file.path, file.ext, file.size, cache)
		})

		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.path, err.Error()))
			errCount++
			continue
		}

		scanned++
	}

	err = tx.Commit()
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to commit batch: %s", err.Error()))
		return 0, 0, len(files)
	}

	return scanned, skipped, errCount
}
