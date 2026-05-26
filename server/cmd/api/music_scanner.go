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
  path  string
  ext   string
  size  int64
  mtime int64
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

    ext := strings.ToLower(helpers.GetFileExtension(path))
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

    batch = append(batch, trackFile{
      path:  path,
      ext:   ext,
      size:  info.Size(),
      mtime: info.ModTime().UnixNano(),
    })

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

  backfilledSpotifyAlbums, err := app.backfillMissingAlbumSpotifyIDs(ctx)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("failed to backfill missing album Spotify matches: %s", err.Error()))
    errorCount++
  } else if backfilledSpotifyAlbums > 0 {
    app.Logger.Info(fmt.Sprintf("backfilled %d missing album Spotify matches", backfilledSpotifyAlbums))
  }

  backfilledSpotifyMusicians, err := app.backfillMissingMusicianSpotifyIDs(ctx)
  if err != nil {
    app.Logger.Error(fmt.Sprintf("failed to backfill missing musician Spotify matches: %s", err.Error()))
    errorCount++
  } else if backfilledSpotifyMusicians > 0 {
    app.Logger.Info(fmt.Sprintf("backfilled %d missing musician Spotify matches", backfilledSpotifyMusicians))
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

// ScannerDBMu serializes scanner writes; each changed track commits independently.
func (app *Application) processMusicBatch(ctx context.Context, files []trackFile) (scanned, skipped, errCount int, processed []string) {
  processed = make([]string, 0, len(files))

  for _, file := range files {
    unchanged, err := app.Queries.CheckTrackUnchanged(ctx, database.CheckTrackUnchangedParams{
      FilePath:  file.path,
      Size:      file.size,
      FileMtime: file.mtime,
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

    err = app.processMusicFile(ctx, file)
    if err != nil {
      app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.path, err.Error()))
      app.recordTrackScanError(ctx, file, err)
      errCount++
      continue
    }

    scanned++
    processed = append(processed, file.path)
  }

  return scanned, skipped, errCount, processed
}

func (app *Application) processMusicFile(ctx context.Context, file trackFile) error {
  app.ScannerDBMu.Lock()
  defer app.ScannerDBMu.Unlock()

  tx, err := app.DB.BeginTx(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to start transaction: %w", err)
  }
  defer tx.Rollback()

  qtx := app.Queries.WithTx(tx)

  err = app.processTrackFileWithInfo(ctx, qtx, file)
  if err != nil {
    return err
  }

  err = tx.Commit()
  if err != nil {
    return fmt.Errorf("failed to commit track: %w", err)
  }

  return nil
}

func (app *Application) recordTrackScanError(ctx context.Context, file trackFile, processErr error) {
  app.ScannerDBMu.Lock()
  defer app.ScannerDBMu.Unlock()

  _, scanErr := app.Queries.UpsertTrackScanErrorByPath(ctx, database.UpsertTrackScanErrorByPathParams{
    FilePath:  file.path,
    Size:      file.size,
    FileMtime: file.mtime,
    ScanError: sql.NullString{String: processErr.Error(), Valid: true},
  })
  if scanErr != nil {
    app.Logger.Warn("failed to record track scan error", "path", file.path, "error", scanErr)
  }
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
      trackMusicianIDs, queryErr := qtx.GetMusicianIDsByTrackID(ctx, track.ID)
      if queryErr != nil {
        return queryErr
      }

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
        for _, mID := range trackMusicianIDs {
          seen[mID] = true
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
      } else {
        seen := make(map[int64]bool)
        if track.MusicianID.Valid {
          seen[track.MusicianID.Int64] = true
        }
        for _, mID := range trackMusicianIDs {
          seen[mID] = true
        }
        for mID := range seen {
          deleteErr = app.deleteMusicianIfUnused(ctx, qtx, mID)
          if deleteErr != nil {
            return deleteErr
          }
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

  err = qtx.DeleteMusicSpotifyMatch(ctx, database.DeleteMusicSpotifyMatchParams{
    EntityType: spotifyMatchEntityAlbum,
    EntityID:   albumID,
  })
  if err != nil {
    return err
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

  err = qtx.DeleteMusicSpotifyMatch(ctx, database.DeleteMusicSpotifyMatchParams{
    EntityType: spotifyMatchEntityMusician,
    EntityID:   musicianID,
  })
  if err != nil {
    return err
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
