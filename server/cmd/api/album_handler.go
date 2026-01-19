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

// GetAlbumsAlphabetical returns a paginated list of albums sorted alphabetically.
// Supports query parameters: page (default 1), per_page (default 24, max 48)
func (app *Application) GetAlbumsAlphabetical(w http.ResponseWriter, r *http.Request) {
	// Parse page with default of 1
	page := int64(1)
	if p := r.URL.Query().Get("page"); p != "" {
		parsed, err := strconv.ParseInt(p, 10, 64)
		if err == nil && parsed > 0 {
			page = parsed
		}
	}

	// Parse per_page with default of 24 and max of 48
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

	// Calculate offset from page number
	offset := (page - 1) * perPage

	// Get total count for pagination
	total, err := app.Queries.GetAlbumsCount(r.Context())
	if err != nil {
		app.Logger.Error("failed to get albums count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch albums count"))
		return
	}

	// Get paginated albums
	albums, err := app.Queries.GetAlbumsAlphabetical(r.Context(), database.GetAlbumsAlphabeticalParams{
		Limit:  perPage,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get albums", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch albums"))
		return
	}

	// Calculate total pages
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

// GetLatestAlbums returns the 12 most recently added albums.
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

// GetAlbumDetails returns an album with all its tracks, artists, and genre information.
func (app *Application) GetAlbumDetails(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid album id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get the album
	album, err := app.Queries.GetAlbumByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("album not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get album", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album from server"))
		return
	}

	// Get all tracks for this album
	tracks, err := app.Queries.GetTracksByAlbumID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.Logger.Error("failed to get tracks for album", "error", err, "album_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album tracks from server"))
		return
	}

	// Get all artists/musicians associated with this album
	artists, err := app.Queries.GetMusiciansByAlbumID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get artists for album", "error", err, "album_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album artists from server"))
		return
	}

	// Get all genres for tracks in this album
	trackGenres, err := app.Queries.GetGenresByAlbumID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.Logger.Error("failed to get genres for album", "error", err, "album_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch album genres from server"))
		return
	}

	// Calculate total duration
	var totalDuration int64
	for _, track := range tracks {
		totalDuration += track.Duration
	}

	// Build unique album genres from track genres
	genreSet := make(map[string]struct{})
	for _, g := range trackGenres {
		genreSet[g.Tag] = struct{}{}
	}
	albumGenres := make([]string, 0, len(genreSet))
	for tag := range genreSet {
		albumGenres = append(albumGenres, tag)
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

// DeleteAlbum deletes an album. Associated tracks are cascade deleted by the database.
func (app *Application) DeleteAlbum(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid album id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Verify the album exists and get title for logging
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

	// Delete the album (cascade will delete tracks, musician_albums, and album_genres)
	err = app.Queries.DeleteAlbum(ctx, id)
	if err != nil {
		app.Logger.Error("failed to delete album", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to delete album"))
		return
	}

	app.Logger.Info("album deleted successfully", "id", id, "title", album.Title)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Album deleted successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
