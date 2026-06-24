package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"path/filepath"
	"time"
)

type trackFile struct {
	path string
	ext  string
	size int64
}

func (app *Application) MusicScanLibrary() {
	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		app.Logger.Error("music directory not configured")
		return
	}

	if !tryBeginMusicScan() {
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
	defer finishMusicScan()

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
	batch := make([]trackFile, 0, helpers.SCANNER_BATCH_SIZE)
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

	err = filepath.WalkDir(app.Settings.MusicDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == app.Settings.MusicDir.String {
				return err
			}
			app.Logger.Error(fmt.Sprintf("error walking directory: %s", err.Error()))
			errorCount++
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		ext := helpers.GetFileExtension(path)
		if !helpers.ValidAudioExtensions[ext] {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to get file info for %s: %s", path, err.Error()))
			errorCount++
			return nil
		}
		size := info.Size()

		if scan.trackUnchanged(path, size) {
			tracksSkipped++
			return nil
		}

		batch = append(batch, trackFile{
			path: path,
			ext:  ext,
			size: size,
		})

		if len(batch) >= helpers.SCANNER_BATCH_SIZE {
			flushBatch()
		}

		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("unexpected error walking music directory: %s", err.Error()))
		return
	}

	flushBatch()

	app.Logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
		tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) processMusicBatchWithContext(ctx context.Context, scan *musicScanContext, files []trackFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		if scan.trackUnchanged(file.path, file.size) {
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
			app.Logger.Warn("failed to persist music track", "path", file.path, "error", err)
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

	index := make(map[string]int64, len(rows))
	for _, row := range rows {
		index[filepath.Clean(row.FilePath)] = row.Size
	}

	return index, nil
}

func tryBeginMusicScan() bool {
	musicScanMutex.Lock()
	defer musicScanMutex.Unlock()

	if isMusicScanning {
		return false
	}

	isMusicScanning = true
	return true
}

func finishMusicScan() {
	musicScanMutex.Lock()
	isMusicScanning = false
	musicScanMutex.Unlock()
}
