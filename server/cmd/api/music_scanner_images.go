package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"path/filepath"
	"time"
)

func (app *Application) throttleCoverArtDownload() {
	app.coverArtThrottleMu.Lock()
	defer app.coverArtThrottleMu.Unlock()
	if elapsed := time.Since(app.lastCoverArtDownload); elapsed < helpers.COVER_ART_MIN_INTERVAL {
		time.Sleep(helpers.COVER_ART_MIN_INTERVAL - elapsed)
	}
	app.lastCoverArtDownload = time.Now()
}

func isExternalURL(s string) bool {
	return len(s) >= 4 && (s[:4] == "http")
}

func (app *Application) downloadAlbumCover(ctx context.Context, album *database.Album, imageURL string) {
	if imageURL == "" {
		return
	}
	destPath := filepath.Join(app.Settings.StaticDir, "albums", fmt.Sprintf("%d.jpg", album.ID))
	localPath := fmt.Sprintf("/api/static/albums/%d.jpg", album.ID)
	app.throttleCoverArtDownload()
	if err := helpers.DownloadImage(imageURL, destPath); err == nil {
		_ = app.Queries.UpdateAlbumCover(ctx, database.UpdateAlbumCoverParams{
			Cover: sql.NullString{String: localPath, Valid: true},
			ID:    album.ID,
		})
	} else if app.Logger != nil {
		app.Logger.Debug("album cover download failed", "album_id", album.ID, "title", album.Title, "error", err.Error())
	}
}

func (app *Application) downloadMusicianThumb(ctx context.Context, musician *database.Musician, imageURL string) {
	if imageURL == "" {
		return
	}
	destPath := filepath.Join(app.Settings.StaticDir, "musicians", fmt.Sprintf("%d.jpg", musician.ID))
	localPath := fmt.Sprintf("/api/static/musicians/%d.jpg", musician.ID)
	app.throttleCoverArtDownload()
	if err := helpers.DownloadImage(imageURL, destPath); err == nil {
		_ = app.Queries.UpdateMusicianThumb(ctx, database.UpdateMusicianThumbParams{
			Thumb: sql.NullString{String: localPath, Valid: true},
			ID:    musician.ID,
		})
	} else if app.Logger != nil {
		app.Logger.Debug("musician thumb download failed", "musician_id", musician.ID, "name", musician.Name, "error", err.Error())
	}
}

func (app *Application) downloadAlbumAndMusicianImages(ctx context.Context) {
	albums, err := app.Queries.GetAlbumsNeedingCoverDownload(ctx)
	if err == nil && len(albums) > 0 {
		app.Logger.Info(fmt.Sprintf("downloading cover art for %d albums", len(albums)))
		for i := range albums {
			a := &albums[i]
			var url string
			if a.Cover.Valid && isExternalURL(a.Cover.String) {
				url = a.Cover.String
			}
			if url == "" && a.MusicbrainzID.Valid {
				url = fmt.Sprintf("%s/%s/front-500", helpers.COVER_ART_ARCHIVE_BASE_URL, a.MusicbrainzID.String)
			}
			if url == "" && a.MusicbrainzID.Valid {
				app.throttleCoverArtDownload()
				if u, e := app.MusicBrainz.GetAlbumImageURL(a.MusicbrainzID.String); e == nil {
					url = u
				}
			}
			if url != "" {
				app.downloadAlbumCover(ctx, a, url)
			}
		}
	}

	musicians, err := app.Queries.GetMusiciansNeedingThumbDownload(ctx)
	if err == nil && len(musicians) > 0 {
		app.Logger.Info(fmt.Sprintf("downloading thumb art for %d musicians", len(musicians)))
		for i := range musicians {
			m := &musicians[i]
			var url string
			if m.Thumb.Valid && isExternalURL(m.Thumb.String) {
				url = m.Thumb.String
			}
			if url == "" && m.MusicbrainzID.Valid {
				app.throttleCoverArtDownload()
				if u, e := app.MusicBrainz.GetArtistImageURL(m.MusicbrainzID.String); e == nil {
					url = u
				}
			}
			if url != "" {
				app.downloadMusicianThumb(ctx, m, url)
			}
		}
	}

	// Log how many still have no local image (no image at source, or download failed e.g. 404)
	if stillAlbums, _ := app.Queries.GetAlbumsNeedingCoverDownload(ctx); len(stillAlbums) > 0 {
		app.Logger.Info(fmt.Sprintf("cover art: %d album(s) still have no local image (Cover Art Archive/AudioDB may have no image for that release)", len(stillAlbums)))
	}
	if stillMusicians, _ := app.Queries.GetMusiciansNeedingThumbDownload(ctx); len(stillMusicians) > 0 {
		app.Logger.Info(fmt.Sprintf("thumb art: %d musician(s) still have no local image (AudioDB may have no image for that artist)", len(stillMusicians)))
	}
}
