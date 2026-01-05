package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// trackFile holds path and pre-computed extension to avoid redundant lookups
type trackFile struct {
	path string
	ext  string
}

// ScanMusicLibrary walks through the configured music directory, extracts metadata
// from audio files using ffprobe, and stores track information in the database.
func (app *Application) ScanMusicLibrary() {
	// Add to WaitGroup for graceful shutdown
	if app.Wait != nil {
		app.Wait.Add(1)
		defer app.Wait.Done()
	}

	// Validate music directory is configured
	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		app.Logger.Error("music directory not configured")
		return
	}

	app.Logger.Info(fmt.Sprintf("scanning music directory: %s", app.Settings.MusicDir.String))

	ctx := context.Background()
	errorCount := 0
	tracksScanned := 0
	tracksSkipped := 0
	startTime := time.Now()

	// Batch buffer to collect tracks before processing
	batch := make([]trackFile, 0, helpers.SCANNER_BATCH_SIZE)

	err := filepath.WalkDir(app.Settings.MusicDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			app.Logger.Error(fmt.Sprintf("error walking directory: %s", err.Error()))
			errorCount++
			return nil
		}

		// Skip directories
		if entry.IsDir() {
			return nil
		}

		ext := helpers.GetFileExtension(path)
		if !helpers.ValidAudioExtensions[ext] {
			return nil
		}

		// Add to current batch with pre-computed extension
		batch = append(batch, trackFile{path: path, ext: ext})

		// Process batch when it reaches the batch size
		if len(batch) >= helpers.SCANNER_BATCH_SIZE {
			scanned, skipped, errors := app.processMusicBatch(ctx, batch)
			tracksScanned += scanned
			tracksSkipped += skipped
			errorCount += errors
			batch = batch[:0]
		}

		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("unexpected error walking music directory: %s", err.Error()))
		return
	}

	// Process remaining tracks in the final batch
	if len(batch) > 0 {
		scanned, skipped, errors := app.processMusicBatch(ctx, batch)
		tracksScanned += scanned
		tracksSkipped += skipped
		errorCount += errors
	}

	app.Spotify.ClearAllCaches()

	app.Logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
		tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

// processMusicBatch processes a batch of audio files within a single transaction.
// Uses skip-on-error strategy: failed tracks don't rollback successful ones.
// Optimization: checks path + size first (fast stat), skips unchanged files.
func (app *Application) processMusicBatch(ctx context.Context, files []trackFile) (scanned, skipped, errCount int) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to start transaction: %s", err.Error()))
		return 0, 0, len(files)
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	for _, file := range files {
		// Get file info (size) - fast filesystem stat, no file read
		fileInfo, err := os.Stat(file.path)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to stat %s: %s", file.path, err.Error()))
			errCount++
			continue
		}
		fileSize := fileInfo.Size()

		// Quick check: if track exists with same path AND size, skip (file unchanged)
		_, err = qtx.CheckTrackUnchanged(ctx, database.CheckTrackUnchangedParams{
			FilePath: file.path,
			Size:     fileSize,
		})

		if err == nil {
			// Track exists with same path and size - skip
			skipped++
			continue
		}

		// File is new or size changed - process it
		err = app.processTrackFile(ctx, qtx, file.path, file.ext)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.path, err.Error()))
			errCount++
			continue
		}

		scanned++
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		app.Logger.Error(fmt.Sprintf("failed to commit batch: %s", err.Error()))
		return 0, 0, len(files)
	}

	return scanned, skipped, errCount
}
