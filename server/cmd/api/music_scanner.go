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

type trackFile struct {
  path string
  ext  string
  size int64
}

func (app *Application) MusicScanLibrary() {
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
  batch := make([]trackFile, 0, helpers.SCANNER_BATCH_SIZE)

  err := filepath.WalkDir(app.Settings.MusicDir.String, func(path string, entry fs.DirEntry, err error) error {
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

    batch = append(batch, trackFile{
      path: path,
      ext:  ext,
      size: info.Size(),
    })

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

  if len(batch) > 0 {
    scanned, skipped, errors := app.processMusicBatch(ctx, batch)
    tracksScanned += scanned
    tracksSkipped += skipped
    errorCount += errors
  }

  if app.Spotify != nil {
    app.Spotify.ClearAllCaches()
  }

  app.Logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
    tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) processMusicBatch(ctx context.Context, files []trackFile) (scanned, skipped, errCount int) {
  app.ScannerDBMu.Lock()
  defer app.ScannerDBMu.Unlock()

  for _, file := range files {
    exists, err := app.Queries.CheckTrackExistsByPathAndSize(ctx, database.CheckTrackExistsByPathAndSizeParams{
      FilePath: file.path,
      Size:     file.size,
    })

    if err != nil {
      app.Logger.Error(fmt.Sprintf("failed to check track state for %s: %s", file.path, err.Error()))
      errCount++
      continue
    }

    if exists {
      skipped++
      continue
    }

    tx, err := app.DB.BeginTx(ctx, nil)
    if err != nil {
      app.Logger.Error(fmt.Sprintf("failed to start transaction for %s: %s", file.path, err.Error()))
      errCount++
      continue
    }

    qtx := app.Queries.WithTx(tx)
    _, err = app.processTrackFile(ctx, qtx, file.path, file.ext, file.size)
    if err != nil {
      rollbackErr := tx.Rollback()
      if rollbackErr != nil {
        app.Logger.Warn("failed to rollback music track transaction", "path", file.path, "error", rollbackErr)
      }
      app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.path, err.Error()))
      errCount++
      continue
    }

    err = tx.Commit()
    if err != nil {
      app.Logger.Error(fmt.Sprintf("failed to commit transaction for %s: %s", file.path, err.Error()))
      errCount++
      continue
    }

    scanned++
  }

  return scanned, skipped, errCount
}
