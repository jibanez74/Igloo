package main

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"

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
