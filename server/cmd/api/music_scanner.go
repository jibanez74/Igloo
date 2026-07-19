package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"time"
)

func (app *Application) MusicScanLibrary() {
	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		app.Logger.Info("skipping music library scan: music directory is not configured")
		return
	}

	if !musicScanGuard.TryBegin() {
		app.Logger.Warn("music library scan is already in progress")
		return
	}

	if app.Wait != nil {
		app.Wait.Add(1)
	}
	go app.runMusicScan()
}

func (app *Application) runMusicScan() {
	if app.Wait != nil {
		defer app.Wait.Done()
	}
	defer musicScanGuard.Finish()

	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		app.Logger.Info("skipping music library scan: music directory is not configured")
		return
	}

	app.Logger.Info(fmt.Sprintf("scanning music directory: %s", app.Settings.MusicDir.String))

	ctx := app.scanContext()
	errorCount := 0
	tracksScanned := 0
	tracksSkipped := 0
	startTime := time.Now()
	batch := make([]helpers.ScanFile, 0, scannerBatchSize)
	scanIndex, err := app.loadMusicScanIndex(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to load music scan index: %s", err.Error()))
		return
	}
	scan := newMusicScanContext(scanIndex)
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		scanned, skipped, errors := app.processMusicBatchWithContext(ctx, scan, batch)
		tracksScanned += scanned
		tracksSkipped += skipped
		errorCount += errors
		batch = batch[:0]
	}

	err = helpers.WalkMediaLibraryContext(
		ctx,
		app.Settings.MusicDir.String,
		helpers.ValidAudioExtensions,
		func(err error) {
			app.Logger.Error(err.Error())
			errorCount++
		},
		func(file helpers.ScanFile) error {
			if scan.trackUnchanged(file.Path, file.Size) {
				tracksSkipped++
				return nil
			}

			batch = append(batch, file)

			if len(batch) >= scannerBatchSize {
				flushBatch()
			}

			return nil
		},
	)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			app.Logger.Info("music library scan canceled")
			return
		}
		app.Logger.Error(fmt.Sprintf("unexpected error walking music directory: %s", err.Error()))
		return
	}

	flushBatch()

	app.Logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
		tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) processMusicBatchWithContext(ctx context.Context, scan *musicScanContext, files []helpers.ScanFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		if ctx.Err() != nil {
			return scanned, skipped, errCount
		}

		if scan.trackUnchanged(file.Path, file.Size) {
			skipped++
			continue
		}

		resolved, err := app.resolveTrackFile(ctx, scan, file)
		if err != nil {
			errCount++
			continue
		}

		_, err = app.persistResolvedTrack(ctx, scan, resolved)
		if err != nil {
			app.Logger.Warn("failed to persist music track", "path", file.Path, "error", err)
			errCount++
			continue
		}

		scanned++
	}

	return scanned, skipped, errCount
}

func (app *Application) loadMusicScanIndex(ctx context.Context) (map[string]int64, error) {
	rows, err := app.Queries.ListMusicTrackScanIndex(ctx)
	if err != nil {
		return nil, err
	}

	return helpers.BuildScanIndex(rows, func(row database.ListMusicTrackScanIndexRow) (string, int64) {
		return row.FilePath, row.Size
	}), nil
}
