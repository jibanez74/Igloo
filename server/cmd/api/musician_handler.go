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

// GetMusiciansAlphabetical returns a paginated list of musicians sorted alphabetically.
// Supports query parameters: page (default 1), per_page (default 24, max 48)
func (app *Application) GetMusiciansAlphabetical(w http.ResponseWriter, r *http.Request) {
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
	total, err := app.Queries.GetMusiciansCount(r.Context())
	if err != nil {
		app.Logger.Error("failed to get musicians count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch musicians count"))
		return
	}

	// Get paginated musicians
	musicians, err := app.Queries.GetMusiciansAlphabetical(r.Context(), database.GetMusiciansAlphabeticalParams{
		Limit:  perPage,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get musicians", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch musicians"))
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
			"musicians":   musicians,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetMusicianDetails returns a musician with all their albums, tracks, and genres.
func (app *Application) GetMusicianDetails(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid musician id"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Get the musician
	musician, err := app.Queries.GetMusicianByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("musician not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to get musician", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch musician from server"))
		return
	}

	// Get all albums for this musician (sorted by release date, newest first)
	albums, err := app.Queries.GetAlbumsByMusicianID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get albums for musician", "error", err, "musician_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch musician albums from server"))
		return
	}

	// Get all tracks by this musician (sorted alphabetically by sort_title)
	tracks, err := app.Queries.GetTracksByMusicianID(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		app.Logger.Error("failed to get tracks for musician", "error", err, "musician_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch musician tracks from server"))
		return
	}

	// Get genres for this musician
	genres, err := app.Queries.GetGenresByMusicianID(ctx, id)
	if err != nil {
		app.Logger.Error("failed to get genres for musician", "error", err, "musician_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch musician genres from server"))
		return
	}

	// Extract genre tags
	genreTags := make([]string, len(genres))
	for i, g := range genres {
		genreTags[i] = g.Tag
	}

	// Calculate total duration
	var totalDuration int64
	for _, track := range tracks {
		totalDuration += track.Duration
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"musician":       musician,
			"albums":         albums,
			"tracks":         tracks,
			"genres":         genreTags,
			"total_duration": totalDuration,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
