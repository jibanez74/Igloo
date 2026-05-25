package main

import (
  "context"
  "database/sql"
  "fmt"
  "igloo/cmd/internal/database"
  "igloo/cmd/internal/helpers"
  "io/fs"
  "os"
  "path/filepath"
  "strings"
  "time"
)

type trackFile struct {
  path string
  ext  string
  size int64
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
  scannedPaths := make(map[string]bool)
  processedPaths := make(map[string]bool)

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

    cleanPath := filepath.Clean(path)
    scannedPaths[cleanPath] = true

    info, err := entry.Info()
    if err != nil {
      app.Logger.Error(fmt.Sprintf("failed to get file info for %s: %s", path, err.Error()))
      errorCount++
      return nil
    }

    batch = append(batch, trackFile{path: path, ext: ext, size: info.Size()})

    if len(batch) >= helpers.SCANNER_BATCH_SIZE {
      scanned, skipped, errors, processed := app.processMusicBatch(ctx, batch)
      tracksScanned += scanned
      tracksSkipped += skipped
      errorCount += errors
      for _, path := range processed {
        processedPaths[filepath.Clean(path)] = true
      }
      batch = batch[:0]
    }

    return nil
  })

  if err != nil {
    app.Logger.Error(fmt.Sprintf("unexpected error walking music directory: %s", err.Error()))
    return
  }

  if len(batch) > 0 {
    scanned, skipped, errors, processed := app.processMusicBatch(ctx, batch)
    tracksScanned += scanned
    tracksSkipped += skipped
    errorCount += errors
    for _, path := range processed {
      processedPaths[filepath.Clean(path)] = true
    }
  }

  deletedCount, err := app.reconcileMissingTracks(
    ctx,
    filepath.Clean(app.Settings.MusicDir.String),
    scannedPaths,
    processedPaths,
  )
  if err != nil {
    app.Logger.Error(fmt.Sprintf("failed to reconcile deleted tracks: %s", err.Error()))
    errorCount++
  } else if deletedCount > 0 {
    app.Logger.Info(fmt.Sprintf("removed %d deleted track entries from database", deletedCount))
  }

  backfilledCovers, err := app.backfillMissingAlbumCovers(ctx)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("failed to backfill missing album covers: %s", err.Error()))
    errorCount++
  } else if backfilledCovers > 0 {
    app.Logger.Info(fmt.Sprintf("backfilled %d missing album covers", backfilledCovers))
  }

  if app.Spotify != nil {
    app.Spotify.ClearAllCaches()
  }

  app.Logger.Info(fmt.Sprintf("music scanner completed: %d scanned, %d skipped, %d errors in %s",
    tracksScanned, tracksSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

// ScannerDBMu serializes scanner writes; savepoints keep one bad track from rolling back the batch.
func (app *Application) processMusicBatch(ctx context.Context, files []trackFile) (scanned, skipped, errCount int, processed []string) {
  app.ScannerDBMu.Lock()
  defer app.ScannerDBMu.Unlock()

  tx, err := app.DB.BeginTx(ctx, nil)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("failed to start transaction: %s", err.Error()))
    return 0, 0, len(files), nil
  }
  defer tx.Rollback()

  qtx := app.Queries.WithTx(tx)
  processed = make([]string, 0, len(files))

  for _, file := range files {
    unchanged, err := qtx.CheckTrackUnchanged(ctx, database.CheckTrackUnchangedParams{
      FilePath: file.path,
      Size:     file.size,
    })

    if err != nil {
      app.Logger.Error(fmt.Sprintf("failed to check track state for %s: %s", file.path, err.Error()))
      errCount++
      continue
    }

    if unchanged {
      skipped++
      continue
    }

    savepointName := fmt.Sprintf("sp_track_%d", scanned+skipped+errCount)

    err = manageSavepoint(ctx, tx, savepointName, func() error {
      return app.processTrackFile(ctx, qtx, file.path, file.ext)
    })
    if err != nil {
      app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.path, err.Error()))
      errCount++
      continue
    }

    scanned++
    processed = append(processed, file.path)
  }

  err = tx.Commit()
  if err != nil {
    processedCount := scanned + skipped + errCount
    successCount := 0
    failedCount := processedCount

    app.Logger.Error(fmt.Sprintf(
      "failed to commit batch: %s, processed=%d, succeeded=%d, failed=%d",
      err.Error(), processedCount, successCount, failedCount,
    ))
    return 0, 0, processedCount, nil
  }

  return scanned, skipped, errCount, processed
}

func (app *Application) reconcileMissingTracks(
  ctx context.Context,
  musicRoot string,
  scannedPaths map[string]bool,
  processedPaths map[string]bool,
) (deletedCount int, err error) {
  app.ScannerDBMu.Lock()
  defer app.ScannerDBMu.Unlock()

  tx, err := app.DB.BeginTx(ctx, nil)
  if err != nil {
    return 0, err
  }
  defer tx.Rollback()

  qtx := app.Queries.WithTx(tx)

  tracks, err := qtx.GetTrackScanIndex(ctx)
  if err != nil {
    return 0, err
  }

  for _, track := range tracks {
    cleanPath := filepath.Clean(track.FilePath)
    if processedPaths[cleanPath] {
      continue
    }
    if !isMusicUnderRoot(cleanPath, musicRoot) || scannedPaths[cleanPath] {
      continue
    }

    _, statErr := os.Stat(cleanPath)
    if statErr == nil {
      continue
    }
    if !os.IsNotExist(statErr) {
      app.Logger.Warn("failed to stat track during reconciliation", "path", cleanPath, "error", statErr)
      continue
    }

    savepointName := fmt.Sprintf("sp_reconcile_track_%d", track.ID)
    err = manageSavepoint(ctx, tx, savepointName, func() error {
      deleteErr := qtx.DeleteTrack(ctx, track.ID)
      if deleteErr != nil {
        return deleteErr
      }

      if track.AlbumID.Valid {
        albumMusicianIDs, queryErr := qtx.GetMusicianIDsByAlbumID(ctx, track.AlbumID.Int64)
        if queryErr != nil {
          return queryErr
        }

        deleteErr = app.deleteAlbumIfEmpty(ctx, qtx, track.AlbumID.Int64)
        if deleteErr != nil {
          return deleteErr
        }

        seen := make(map[int64]bool)
        if track.MusicianID.Valid {
          seen[track.MusicianID.Int64] = true
        }
        for _, mID := range albumMusicianIDs {
          if !seen[mID] {
            seen[mID] = true
          }
        }
        for mID := range seen {
          deleteErr = app.deleteMusicianIfUnused(ctx, qtx, mID)
          if deleteErr != nil {
            return deleteErr
          }
        }
      } else if track.MusicianID.Valid {
        deleteErr = app.deleteMusicianIfUnused(ctx, qtx, track.MusicianID.Int64)
        if deleteErr != nil {
          return deleteErr
        }
      }

      deletedCount++
      return nil
    })
    if err != nil {
      return deletedCount, err
    }
  }

  err = tx.Commit()
  if err != nil {
    return deletedCount, err
  }

  return deletedCount, nil
}

func (app *Application) deleteAlbumIfEmpty(ctx context.Context, qtx *database.Queries, albumID int64) error {
  trackCount, err := qtx.CountTracksByAlbumID(ctx, sqlNullInt64(albumID))
  if err != nil {
    return err
  }
  if trackCount > 0 {
    return nil
  }

  return qtx.DeleteAlbum(ctx, albumID)
}

func (app *Application) deleteMusicianIfUnused(ctx context.Context, qtx *database.Queries, musicianID int64) error {
  trackCount, err := qtx.CountTracksByMusicianID(ctx, sqlNullInt64(musicianID))
  if err != nil {
    return err
  }
  if trackCount > 0 {
    return nil
  }

  albumCount, err := qtx.CountAlbumsByMusicianID(ctx, musicianID)
  if err != nil {
    return err
  }
  if albumCount > 0 {
    return nil
  }

  return qtx.DeleteMusician(ctx, musicianID)
}

func isMusicUnderRoot(path, root string) bool {
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

func sqlNullInt64(value int64) sql.NullInt64 {
  return sql.NullInt64{
    Int64: value,
    Valid: true,
  }
}
