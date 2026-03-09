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

type pendingTrack struct {
	prepared *preparedTrack
	file     trackFile
}

// unchangedTrackPathsAndSizes returns a map of file_path -> size for tracks that exist in the DB (batch query).
// Callers use it to skip files where unchanged[path] == file.size.
func (app *Application) unchangedTrackPathsAndSizes(ctx context.Context, files []trackFile) map[string]int64 {
  if len(files) == 0 {
    return nil
  }
  paths := make([]string, 0, len(files))
  for _, f := range files {
    paths = append(paths, f.path)
  }
  rows, err := app.Queries.GetTrackPathsAndSizesByPaths(ctx, paths)
  if err != nil {
    return nil
  }
  out := make(map[string]int64, len(rows))
  for _, r := range rows {
    out[r.FilePath] = r.Size
  }
  return out
}

func (app *Application) ScanMusicLibrary() {
  if app.Wait != nil {
    app.Wait.Add(1)
    defer app.Wait.Done()
  }

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

    batch = append(batch, trackFile{path: path, ext: ext, size: info.Size()})

    // Process batch when full
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

  if app.Settings.DownloadImages {
    app.downloadAlbumAndMusicianImages(ctx)
  }

  app.MusicBrainz.ClearAllCaches()

  app.Logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
    tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) processMusicBatch(ctx context.Context, files []trackFile) (scanned, skipped, errCount int) {
	var pending []pendingTrack

	unchanged := app.unchangedTrackPathsAndSizes(ctx, files)
	for _, file := range files {
		if size, ok := unchanged[file.path]; ok && size == file.size {
			skipped++
			continue
		}
		prepared, err := app.extractTrackMetadata(ctx, file.path, file.ext)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to extract metadata %s: %s", file.path, err.Error()))
			errCount++
			continue
		}
		pending = append(pending, pendingTrack{prepared: prepared, file: file})
	}

	if len(pending) == 0 {
		return scanned, skipped, errCount
	}

	app.ScannerDBMu.Lock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		app.ScannerDBMu.Unlock()
		app.Logger.Error(fmt.Sprintf("failed to start transaction: %s", err.Error()))
		return 0, skipped, len(pending) + errCount
	}

	qtx := app.Queries.WithTx(tx)

	for _, p := range pending {
		err := app.writeTrackToDB(ctx, qtx, p.prepared)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to write %s: %s", p.file.path, err.Error()))
			errCount++
			continue
		}
		scanned++
	}

	if commitErr := tx.Commit(); commitErr != nil {
		app.Logger.Error(fmt.Sprintf("failed to commit batch: %s", commitErr.Error()))
		app.ScannerDBMu.Unlock()
		return 0, skipped, len(pending) + errCount
	}

	app.ScannerDBMu.Unlock()

	return scanned, skipped, errCount
}
