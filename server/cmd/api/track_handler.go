package main

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// GetTrackByID returns a single track by its primary key ID.
func (app *Application) GetTrackByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid track id"), http.StatusBadRequest)
		return
	}

	track, err := app.Queries.GetTrack(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("track not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get track", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch track from server"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"track": track,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// StreamTrack streams the audio file for playback.
// Uses http.ServeContent which handles:
//   - Range requests (for seeking/scrubbing)
//   - If-Modified-Since headers (caching)
//   - Content-Type and Content-Length headers
func (app *Application) StreamTrack(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid track id"), http.StatusBadRequest)
		return
	}

	track, err := app.Queries.GetTrack(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("track not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to get track for streaming", "error", err, "id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch track from server"))
		return
	}

	file, err := os.Open(track.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			app.Logger.Error("track file not found on disk", "path", track.FilePath, "id", id)
			helpers.ErrorJSON(w, errors.New("track file not found"), http.StatusNotFound)
			return
		}

		app.Logger.Error("failed to open track file", "error", err, "path", track.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to open track file"))
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		app.Logger.Error("failed to stat track file", "error", err, "path", track.FilePath)
		helpers.ErrorJSON(w, errors.New("failed to read track file"))
		return
	}

	w.Header().Set("Content-Type", track.MimeType)

	http.ServeContent(w, r, track.FileName, stat.ModTime(), file)
}

// GetTracksAlphabetical returns a paginated list of tracks sorted alphabetically.
// Supports query parameters: limit (default 50, max 100), offset (default 0)
func (app *Application) GetTracksAlphabetical(w http.ResponseWriter, r *http.Request) {
	// Parse limit with default of 50 and max of 100
	limit := int64(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.ParseInt(l, 10, 64)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	// Parse offset with default of 0
	offset := int64(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		parsed, err := strconv.ParseInt(o, 10, 64)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get total count for pagination
	total, err := app.Queries.GetTracksCount(r.Context())
	if err != nil {
		app.Logger.Error("failed to get tracks count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch tracks count"))
		return
	}

	// Get paginated tracks
	tracks, err := app.Queries.GetTracksAlphabetical(r.Context(), database.GetTracksAlphabeticalParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		app.Logger.Error("failed to get tracks", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch tracks"))
		return
	}

	hasMore := offset+limit < total

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"tracks":   tracks,
			"total":    total,
			"offset":   offset,
			"limit":    limit,
			"has_more": hasMore,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetMusicStats returns the total counts of albums, tracks, and musicians.
func (app *Application) GetMusicStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	albumsCount, err := app.Queries.GetAlbumsCount(ctx)
	if err != nil {
		app.Logger.Error("failed to get albums count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch music stats"))
		return
	}

	tracksCount, err := app.Queries.GetTracksCount(ctx)
	if err != nil {
		app.Logger.Error("failed to get tracks count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch music stats"))
		return
	}

	musiciansCount, err := app.Queries.GetMusiciansCount(ctx)
	if err != nil {
		app.Logger.Error("failed to get musicians count", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch music stats"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"total_albums":    albumsCount,
			"total_tracks":    tracksCount,
			"total_musicians": musiciansCount,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
