package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (app *Application) GetAlbumsAlphabetical(w http.ResponseWriter, r *http.Request) {
	page := int64(1)
	if p := r.URL.Query().Get("page"); p != "" {
		parsed, err := strconv.ParseInt(p, 10, 64)
		if err == nil && parsed > 0 {
			page = parsed
		}
	}

	perPage := int64(24)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		parsed, err := strconv.ParseInt(pp, 10, 64)
		if err == nil && parsed > 0 {
			perPage = parsed
		}
	}

	if perPage > 48 {
		perPage = 48
	}

	offset := (page - 1) * perPage

	total, err := app.Queries.GetAlbumsCount(r.Context())
	if err != nil {
		app.Logger.Error("failed to get albums count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch albums count"))
		return
	}

	albums, err := app.Queries.GetAlbumsAlphabetical(r.Context(), database.GetAlbumsAlphabeticalParams{
		Limit:  perPage,
		Offset: offset,
	})

	if err != nil {
		app.Logger.Error("failed to get albums", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch albums"))
		return
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"albums":      albums,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetLatestAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := app.Queries.GetLatestAlbums(r.Context())
	if err != nil {
		app.Logger.Error("failed to get latest albums", "error", err)
		helpers.ErrorJSON(w, errors.New("fail to fetch latest albums from server"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"albums": albums,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// Use a read-only transaction so album details come from one snapshot.
func (app *Application) GetAlbumDetails(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid album id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		app.Logger.Error("failed to begin transaction", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch album from server"))
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

		app.Logger.Error("failed to get album", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album from server"))
		return
	}

	tracks, err := qtx.GetTracksByAlbumID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.Logger.Error("failed to get tracks for album", "error", err, "album_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album tracks from server"))
		return
	}

	artists, err := qtx.GetMusiciansByAlbumID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get artists for album", "error", err, "album_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album artists from server"))
		return
	}

	trackGenres, err := qtx.GetGenresByAlbumID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.Logger.Error("failed to get genres for album", "error", err, "album_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album genres from server"))
		return
	}

	albumGenreRows, err := qtx.GetAlbumGenres(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get album-level genres", "error", err, "album_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album genres from server"))
		return
	}

	var totalDuration int64

	for _, track := range tracks {
		totalDuration += track.Duration
	}

	albumGenres := make([]string, 0, len(albumGenreRows))
	for _, g := range albumGenreRows {
		albumGenres = append(albumGenres, g.Tag)
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"album":          album,
			"tracks":         tracks,
			"artists":        artists,
			"track_genres":   trackGenres,
			"album_genres":   albumGenres,
			"total_duration": totalDuration,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// Track and join rows are cascade-deleted by the database.
func (app *Application) DeleteAlbum(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid album id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	album, err := app.Queries.GetAlbumByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("album not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get album for deletion", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to verify album exists"))
		return
	}

	err = app.Queries.DeleteAlbum(ctx, id)
	if err != nil {
		app.Logger.Error("failed to delete album", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to delete album"))
		return
	}

	// The album cascades to its tracks (schema.sql, tracks.album_id ON DELETE
	// CASCADE), so which stream-file keys just died is unknown. Drop them all;
	// after the delete, so no racing lookup can republish a removed track.
	app.StreamFileCache.invalidateAll()

	app.Logger.Info("album deleted successfully", "id", id, "title", album.Title)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Album deleted successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
