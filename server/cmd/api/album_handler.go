package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
)

func (app *Application) GetAlbumsAlphabetical(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePageAndPerPage(r, libraryDefaultPerPage, libraryMaxPerPage)

	offset := (page - 1) * perPage

	total, err := app.Queries.GetAlbumsCount(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get albums count", "failed to fetch albums count")
		return
	}

	albums, err := app.Queries.GetAlbumsAlphabetical(r.Context(), database.GetAlbumsAlphabeticalParams{
		Limit:  perPage,
		Offset: offset,
	})

	if err != nil {
		app.serverError(w, err, "failed to get albums", "failed to fetch albums")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"albums":      albums,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages(total, perPage),
		},
	})
}

func (app *Application) GetLatestAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := app.Queries.GetLatestAlbums(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to get latest albums", "fail to fetch latest albums from server")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"albums": albums,
		},
	})
}

// Use a read-only transaction so album details come from one snapshot.
func (app *Application) GetAlbumDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", "invalid album id")
	if !ok {
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.serverError(w, err, beginTransactionLogMessage, "failed to fetch album from server")
		return
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	album, err := qtx.GetAlbumByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("album not found"), http.StatusNotFound)
			return
		}

		app.serverError(w, err, "failed to get album", "failed to fetch album from server", "id", id)
		return
	}

	// A query argument for a nullable foreign key, not a value being stored:
	// helpers.NullInt64 would turn an id of 0 into a NULL scan matching nothing.
	tracks, err := qtx.GetTracksByAlbumID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.serverError(w, err, "failed to get tracks for album", "failed to fetch album tracks from server", "album_id", id)
		return
	}

	artists, err := qtx.GetMusiciansByAlbumID(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get artists for album", "failed to fetch album artists from server", "album_id", id)
		return
	}

	trackGenres, err := qtx.GetGenresByAlbumID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.serverError(w, err, "failed to get genres for album", "failed to fetch album genres from server", "album_id", id)
		return
	}

	albumGenres, err := qtx.GetAlbumGenres(ctx, id)
	if err != nil {
		app.serverError(w, err, "failed to get album-level genres", "failed to fetch album genres from server", "album_id", id)
		return
	}

	var totalDuration int64

	for _, track := range tracks {
		totalDuration += track.Duration
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"album":          album,
			"tracks":         tracks,
			"artists":        artists,
			"track_genres":   trackGenres,
			"album_genres":   albumGenres,
			"total_duration": totalDuration,
		},
	})
}

// Track and join rows are cascade-deleted by the database.
func (app *Application) DeleteAlbum(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r, "id", "invalid album id")
	if !ok {
		return
	}

	// The delete returns the title, so no row doubles as the existence check.
	title, err := app.Queries.DeleteAlbum(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("album not found"), http.StatusNotFound)
			return
		}

		app.serverError(w, err, "failed to delete album", "failed to delete album", "id", id)
		return
	}

	// The album cascades to its tracks (schema.sql, tracks.album_id ON DELETE
	// CASCADE), so which stream-file keys just died is unknown. Drop them all;
	// after the delete, so no racing lookup can republish a removed track.
	app.StreamFileCache.invalidateAll()

	app.Logger.Info("album deleted successfully", "id", id, "title", title)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "Album deleted successfully"})
}
