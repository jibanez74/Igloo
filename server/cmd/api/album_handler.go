package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

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
