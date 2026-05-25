package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const albumArtworkExtractionTimeout = 30 * time.Second

var albumArtworkNames = []string{"cover", "folder", "front", "album"}
var albumArtworkExtensions = []string{".jpg", ".jpeg", ".png", ".webp"}

type albumScanInput struct {
	Title            string
	SortTitle        string
	AlbumArtist      string
	Year             int
	TrackTitles      []string
	TrackPaths       []string
	CurrentTrackPath string
	CurrentMetadata  *ffprobe.FfprobeResult
}

func albumCoverMissing(album database.Album) bool {
	if !album.Cover.Valid {
		return true
	}

	return strings.TrimSpace(album.Cover.String) == ""
}

func (app *Application) resolveAlbumCoverIfMissing(
	ctx context.Context,
	qtx *database.Queries,
	album database.Album,
	spotifyCoverURL string,
	input albumScanInput,
) (database.Album, error) {
	if !albumCoverMissing(album) {
		return album, nil
	}

	if spotifyCoverURL != "" {
		updated, err := app.setSpotifyAlbumCoverIfMissing(ctx, qtx, album.ID, spotifyCoverURL)
		if err != nil {
			return album, err
		}
		if updated {
			return qtx.GetAlbumByID(ctx, album.ID)
		}
	}

	updated, err := app.setLocalAlbumCoverIfMissing(ctx, qtx, album.ID, input)
	if err != nil {
		return album, err
	}
	if updated {
		return qtx.GetAlbumByID(ctx, album.ID)
	}

	return album, nil
}

func (app *Application) backfillMissingAlbumCovers(ctx context.Context) (int, error) {
	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	albums, err := qtx.GetAlbumsMissingCover(ctx)
	if err != nil {
		return 0, err
	}

	backfilled := 0
	for index, album := range albums {
		savepointName := fmt.Sprintf("sp_album_cover_%d", index)
		err = manageSavepoint(ctx, tx, savepointName, func() error {
			updated, updateErr := app.backfillMissingAlbumCover(ctx, qtx, album)
			if updateErr != nil {
				return updateErr
			}
			if updated {
				backfilled++
			}
			return nil
		})
		if err != nil {
			app.Logger.Warn("failed to backfill album cover", "album_id", album.ID, "title", album.Title, "error", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return backfilled, nil
}

func (app *Application) backfillMissingAlbumCover(
	ctx context.Context,
	qtx *database.Queries,
	album database.Album,
) (bool, error) {
	tracks, err := qtx.GetAlbumTracksForArtwork(ctx, sql.NullInt64{Int64: album.ID, Valid: true})
	if err != nil {
		return false, err
	}
	if len(tracks) == 0 {
		return false, nil
	}

	input := albumScanInput{
		Title:     album.Title,
		SortTitle: album.SortTitle,
		Year:      int(album.Year.Int64),
	}
	if album.Musician.Valid {
		input.AlbumArtist = album.Musician.String
	}

	seenTitles := make(map[string]struct{}, len(tracks))
	seenPaths := make(map[string]struct{}, len(tracks))
	for _, track := range tracks {
		title := strings.TrimSpace(track.Title)
		titleKey := strings.ToLower(title)
		if title != "" {
			if _, exists := seenTitles[titleKey]; !exists {
				input.TrackTitles = append(input.TrackTitles, title)
				seenTitles[titleKey] = struct{}{}
			}
		}

		trackPath := strings.TrimSpace(track.FilePath)
		if trackPath != "" {
			if _, exists := seenPaths[trackPath]; !exists {
				input.TrackPaths = append(input.TrackPaths, trackPath)
				seenPaths[trackPath] = struct{}{}
			}
		}

		if input.Year == 0 && track.Year.Valid {
			input.Year = int(track.Year.Int64)
		}
	}

	_, err = app.getOrCreateAlbum(ctx, qtx, input)
	if err != nil {
		return false, err
	}

	updatedAlbum, err := qtx.GetAlbumByID(ctx, album.ID)
	if err != nil {
		return false, err
	}

	return !albumCoverMissing(updatedAlbum), nil
}

func (app *Application) setSpotifyAlbumCoverIfMissing(
	ctx context.Context,
	qtx *database.Queries,
	albumID int64,
	coverURL string,
) (bool, error) {
	cover := strings.TrimSpace(coverURL)
	if cover == "" {
		return false, nil
	}

	if app.Settings != nil && app.Settings.DownloadImages {
		localURL, err := app.downloadAlbumArtwork(albumID, cover)
		if err != nil {
			app.Logger.Warn("failed to download Spotify album artwork", "album_id", albumID, "error", err)
		} else {
			cover = localURL
		}
	}

	rows, err := qtx.UpdateAlbumCoverIfMissing(ctx, database.UpdateAlbumCoverIfMissingParams{
		Cover: sql.NullString{String: cover, Valid: true},
		ID:    albumID,
	})
	if err != nil {
		return false, err
	}

	return rows > 0, nil
}

func (app *Application) setLocalAlbumCoverIfMissing(
	ctx context.Context,
	qtx *database.Queries,
	albumID int64,
	input albumScanInput,
) (bool, error) {
	for _, trackPath := range input.TrackPaths {
		sourcePath, ext, ok := findFolderAlbumArtwork(trackPath)
		if !ok {
			continue
		}

		localURL, err := app.copyAlbumArtworkFile(albumID, sourcePath, ext)
		if err != nil {
			app.Logger.Warn("failed to copy local album artwork", "album_id", albumID, "path", sourcePath, "error", err)
			continue
		}

		updated, err := updateAlbumCoverIfMissing(ctx, qtx, albumID, localURL)
		if err != nil {
			return false, err
		}
		if updated {
			return true, nil
		}

		return false, nil
	}

	if app.FFmpeg == nil {
		return false, nil
	}

	for _, trackPath := range input.TrackPaths {
		metadata := input.CurrentMetadata
		if trackPath != input.CurrentTrackPath {
			if app.Ffprobe == nil {
				continue
			}

			probedMetadata, err := app.Ffprobe.GetMetadata(trackPath)
			if err != nil {
				app.Logger.Warn("failed to probe track for embedded album artwork", "path", trackPath, "error", err)
				continue
			}

			metadata = probedMetadata
		}

		streamIndex, ok := findEmbeddedAlbumArtworkStream(metadata)
		if !ok {
			continue
		}

		extractCtx, cancel := context.WithTimeout(ctx, albumArtworkExtractionTimeout)
		imageBytes, err := app.FFmpeg.ExtractAudioImage(extractCtx, trackPath, streamIndex)
		cancel()
		if err != nil {
			app.Logger.Warn("failed to extract embedded album artwork", "album_id", albumID, "path", trackPath, "error", err)
			continue
		}

		localURL, err := app.writeAlbumArtworkBytes(albumID, ".jpg", imageBytes)
		if err != nil {
			app.Logger.Warn("failed to write embedded album artwork", "album_id", albumID, "path", trackPath, "error", err)
			continue
		}

		updated, err := updateAlbumCoverIfMissing(ctx, qtx, albumID, localURL)
		if err != nil {
			return false, err
		}
		if updated {
			return true, nil
		}

		return false, nil
	}

	return false, nil
}

func updateAlbumCoverIfMissing(ctx context.Context, qtx *database.Queries, albumID int64, coverURL string) (bool, error) {
	rows, err := qtx.UpdateAlbumCoverIfMissing(ctx, database.UpdateAlbumCoverIfMissingParams{
		Cover: sql.NullString{String: coverURL, Valid: true},
		ID:    albumID,
	})
	if err != nil {
		return false, err
	}

	return rows > 0, nil
}

func findFolderAlbumArtwork(trackPath string) (string, string, bool) {
	entries, err := os.ReadDir(filepath.Dir(trackPath))
	if err != nil {
		return "", "", false
	}

	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		files[strings.ToLower(entry.Name())] = entry.Name()
	}

	for _, name := range albumArtworkNames {
		for _, ext := range albumArtworkExtensions {
			fileName := name + ext
			actualName, ok := files[fileName]
			if !ok {
				continue
			}

			return filepath.Join(filepath.Dir(trackPath), actualName), ext, true
		}
	}

	return "", "", false
}

func findEmbeddedAlbumArtworkStream(metadata *ffprobe.FfprobeResult) (int64, bool) {
	if metadata == nil {
		return 0, false
	}

	for _, stream := range metadata.Streams {
		if stream.Disposition.AttachedPic == 1 {
			return int64(stream.Index), true
		}
	}

	for _, stream := range metadata.Streams {
		if stream.CodecType == "video" && helpers.IsCoverArtVideoCodec(stream.CodecName) {
			return int64(stream.Index), true
		}
	}

	return 0, false
}

func (app *Application) downloadAlbumArtwork(albumID int64, sourceURL string) (string, error) {
	destPath, staticURL, err := app.albumArtworkPath(albumID, ".jpg")
	if err != nil {
		return "", err
	}

	err = helpers.DownloadImage(sourceURL, destPath)
	if err != nil {
		return "", err
	}

	return staticURL, nil
}

func (app *Application) copyAlbumArtworkFile(albumID int64, sourcePath, ext string) (string, error) {
	destPath, staticURL, err := app.albumArtworkPath(albumID, ext)
	if err != nil {
		return "", err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	if err != nil {
		_ = os.Remove(destPath)
		return "", err
	}

	return staticURL, nil
}

func (app *Application) writeAlbumArtworkBytes(albumID int64, ext string, imageBytes []byte) (string, error) {
	destPath, staticURL, err := app.albumArtworkPath(albumID, ext)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(destPath, imageBytes, 0o644)
	if err != nil {
		return "", err
	}

	return staticURL, nil
}

func (app *Application) albumArtworkPath(albumID int64, ext string) (string, string, error) {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	if ext != ".jpg" && ext != ".png" && ext != ".webp" {
		return "", "", fmt.Errorf("unsupported album artwork extension: %s", ext)
	}

	if app.Settings == nil || strings.TrimSpace(app.Settings.StaticDir) == "" {
		return "", "", fmt.Errorf("static directory is not configured")
	}

	albumsDir := filepath.Join(app.Settings.StaticDir, "albums")
	_, err := helpers.GetOrCreateDir(albumsDir)
	if err != nil {
		return "", "", err
	}

	filename := fmt.Sprintf("album-%d%s", albumID, ext)
	return filepath.Join(albumsDir, filename), "/api/static/albums/" + filename, nil
}
