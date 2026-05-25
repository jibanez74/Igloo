package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	spotifyapi "igloo/cmd/internal/spotify"
	"strings"
)

func (app *Application) backfillMissingAlbumSpotifyIDs(ctx context.Context) (int, error) {
	if app.Spotify == nil {
		return 0, nil
	}

	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	albums, err := qtx.GetAlbumsMissingSpotifyID(ctx)
	if err != nil {
		return 0, err
	}

	backfilled := 0
	for index, album := range albums {
		savepointName := fmt.Sprintf("sp_album_spotify_%d", index)
		err = manageSavepoint(ctx, tx, savepointName, func() error {
			updated, updateErr := app.backfillMissingAlbumSpotifyID(ctx, qtx, album)
			if updateErr != nil {
				return updateErr
			}
			if updated {
				backfilled++
			}
			return nil
		})
		if err != nil {
			app.logSpotifyAlbumMatchFailure(album, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return backfilled, nil
}

func (app *Application) backfillMissingAlbumSpotifyID(
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

	input := buildExistingAlbumScanInput(album, tracks)
	_, err = app.getOrCreateSpotifyAlbum(ctx, qtx, input)
	if err != nil {
		return false, err
	}

	updatedAlbum, err := qtx.GetAlbumByID(ctx, album.ID)
	if err != nil {
		return false, err
	}

	return spotifyIDPresent(updatedAlbum.SpotifyID), nil
}

func spotifyIDPresent(spotifyID sql.NullString) bool {
	return spotifyID.Valid && strings.TrimSpace(spotifyID.String) != ""
}

func (app *Application) logSpotifyAlbumMatchFailure(album database.Album, err error) {
	if app.Logger == nil {
		return
	}

	artist := ""
	if album.Musician.Valid {
		artist = album.Musician.String
	}

	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		app.Logger.Warn("failed to match album on Spotify",
			"album_id", album.ID,
			"title", album.Title,
			"artist", artist,
			"error", err,
		)
		return
	}

	info := matchErr.Info
	args := []any{
		"album_id", album.ID,
		"title", album.Title,
		"artist", artist,
		"search", info.SearchQuery,
		"strategy", info.Strategy,
		"reason", info.Reason,
		"candidate", info.CandidateName,
		"candidate_artist", info.CandidateArtist,
		"score", info.Score,
		"threshold", info.Threshold,
	}
	if matchErr.Err != nil {
		args = append(args, "error", matchErr.Err)
	}

	app.Logger.Warn("failed to match album on Spotify", args...)
}
