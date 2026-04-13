package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type movieFile struct {
	path string
	ext  string
	size int64
}

func (app *Application) ScanMoviesLibrary() {
	if app.Wait != nil {
		app.Wait.Add(1)
		defer app.Wait.Done()
	}

	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		app.Logger.Error("movies directory not configured")
		return
	}

	app.Logger.Info(fmt.Sprintf("scanning movies directory: %s", app.Settings.MoviesDir.String))

	ctx := context.Background()
	errorCount := 0
	moviesScanned := 0
	moviesSkipped := 0
	startTime := time.Now()
	scannedPaths := make(map[string]bool)

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

		cleanPath := filepath.Clean(path)
		scannedPaths[cleanPath] = true

		info, err := entry.Info()
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to get file info for %s: %s", path, err.Error()))
			errorCount++
			return nil
		}

		batch = append(batch, movieFile{path: path, ext: ext, size: info.Size()})

		// Process batch when full
		if len(batch) >= helpers.SCANNER_BATCH_SIZE {
			scanned, skipped, errors := app.processMoviesBatch(ctx, batch)
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
		scanned, skipped, errors := app.processMoviesBatch(ctx, batch)
		moviesScanned += scanned
		moviesSkipped += skipped
		errorCount += errors
	}

	reconciled, err := app.reconcileMissingMovies(ctx, filepath.Clean(app.Settings.MoviesDir.String), scannedPaths)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to reconcile deleted movies: %s", err.Error()))
		errorCount++
	} else if reconciled > 0 {
		app.Logger.Info(fmt.Sprintf("removed %d deleted movie entries from database", reconciled))
	}

	app.Logger.Info(fmt.Sprintf("movies scanner completed: %d scanned, %d skipped, %d errors in %s",
		moviesScanned, moviesSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

// processMoviesBatch processes a batch of movie files within a single transaction.
// Uses skip-on-error strategy: failed movies don't rollback successful ones.
// Holds ScannerDBMu so only one scanner (music or movie) writes to the DB at a time.
func (app *Application) processMoviesBatch(ctx context.Context, files []movieFile) (scanned, skipped, errCount int) {
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

			return app.processMovieFile(ctx, qtx, file.path, file.ext, file.size)
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

func (app *Application) reconcileMissingMovies(ctx context.Context, moviesRoot string, scannedPaths map[string]bool) (int, error) {
	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	movies, err := qtx.GetMovieScanIndex(ctx)
	if err != nil {
		return 0, err
	}

	deletedCount := 0
	for _, movie := range movies {
		cleanPath := filepath.Clean(movie.FilePath)
		if !isMovieUnderRoot(cleanPath, moviesRoot) {
			continue
		}
		if scannedPaths[cleanPath] {
			continue
		}

		_, statErr := os.Stat(cleanPath)
		if statErr == nil {
			continue
		}
		if !os.IsNotExist(statErr) {
			app.Logger.Warn("failed to stat movie during reconciliation", "path", cleanPath, "error", statErr)
			continue
		}

		app.invalidateSubtitleVTTCache(movie.ID)

		err = qtx.DeleteMovie(ctx, movie.ID)
		if err != nil {
			return deletedCount, err
		}

		deletedCount++
	}

	err = tx.Commit()
	if err != nil {
		return deletedCount, err
	}

	return deletedCount, nil
}

func isMovieUnderRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == "" || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
