package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"path/filepath"
)

func (app *Application) acquireAlbumCover(ctx context.Context, qtx *database.Queries, album *database.Album, coverURL, audioFilePath string) {
	if album.Cover.Valid && !isExternalURL(album.Cover.String) {
		return
	}

	destPath := filepath.Join(app.Settings.StaticDir, "albums", fmt.Sprintf("%d.jpg", album.ID))
	localPath := fmt.Sprintf("/api/static/albums/%d.jpg", album.ID)

	if coverURL == "" && album.MusicbrainzID.Valid {
		coverURL = fmt.Sprintf("https://coverartarchive.org/release-group/%s/front-500", album.MusicbrainzID.String)
	}

	if coverURL != "" {
		if err := helpers.DownloadImage(coverURL, destPath); err == nil {
			qtx.UpdateAlbumCover(ctx, database.UpdateAlbumCoverParams{
				Cover: sql.NullString{String: localPath, Valid: true},
				ID:    album.ID,
			})
			return
		}
	}

	if album.MusicbrainzID.Valid {
		if audioDBURL, err := app.MusicBrainz.GetAlbumImageURL(album.MusicbrainzID.String); err == nil {
			if err := helpers.DownloadImage(audioDBURL, destPath); err == nil {
				qtx.UpdateAlbumCover(ctx, database.UpdateAlbumCoverParams{
					Cover: sql.NullString{String: localPath, Valid: true},
					ID:    album.ID,
				})
				return
			}
		}
	}

	if app.FFmpeg != nil && audioFilePath != "" {
		if err := app.FFmpeg.ExtractEmbeddedArt(audioFilePath, destPath); err == nil {
			qtx.UpdateAlbumCover(ctx, database.UpdateAlbumCoverParams{
				Cover: sql.NullString{String: localPath, Valid: true},
				ID:    album.ID,
			})
		}
	}
}

func isExternalURL(s string) bool {
	return len(s) >= 4 && (s[:4] == "http")
}

func (app *Application) retryMissingAlbumCovers(ctx context.Context) {
	albums, err := app.Queries.GetAlbumsWithMissingCovers(ctx)
	if err != nil || len(albums) == 0 {
		return
	}
	app.Logger.Info(fmt.Sprintf("retrying covers for %d albums with MusicBrainz IDs", len(albums)))
	for i := range albums {
		app.acquireAlbumCover(ctx, app.Queries, &albums[i], "", "")
	}
}

func (app *Application) acquireMusicianThumb(ctx context.Context, qtx *database.Queries, musician *database.Musician) {
	if musician.Thumb.Valid || !musician.MusicbrainzID.Valid {
		return
	}

	imageURL, err := app.MusicBrainz.GetArtistImageURL(musician.MusicbrainzID.String)
	if err != nil {
		return
	}

	destPath := filepath.Join(app.Settings.StaticDir, "musicians", fmt.Sprintf("%d.jpg", musician.ID))
	localPath := fmt.Sprintf("/api/static/musicians/%d.jpg", musician.ID)

	err = helpers.DownloadImage(imageURL, destPath)
	if err == nil {
		qtx.UpdateMusicianThumb(ctx, database.UpdateMusicianThumbParams{
			Thumb: sql.NullString{String: localPath, Valid: true},
			ID:    musician.ID,
		})
	}
}
