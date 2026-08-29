package main

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

const watchCompletionThreshold = 0.98

type updateMovieWatchProgressRequest struct {
	ProgressSec   *float64 `json:"progress_sec"`
	DurationSec   *float64 `json:"duration_sec"`
	SaveSessionID *string  `json:"save_session_id"`
	SaveSequence  *int64   `json:"save_sequence"`
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}

	hexValue := strings.ReplaceAll(value, "-", "")
	_, err := hex.DecodeString(hexValue)
	return err == nil
}

type setMovieWatchedRequest struct {
	Watched *bool `json:"watched"`
}

type movieWatchProgressResponse struct {
	ProgressSec *float64 `json:"progress_sec"`
	DurationSec *float64 `json:"duration_sec"`
	Watched     bool     `json:"watched"`
	UpdatedAt   *string  `json:"updated_at"`
}

func parseMovieID(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	movieID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || movieID <= 0 {
		return 0, errors.New("invalid movie id")
	}
	return movieID, nil
}

func (app *Application) ensureMovieExists(r *http.Request, movieID int64) error {
	exists, err := app.Queries.MovieExists(r.Context(), movieID)
	if err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

// rejectWatchProgressWrite turns a failed movie_watch_progress write into a
// response. The movie_id foreign key already rejects an unknown movie, so the
// write paths do not pre-check existence -- they pay one query on success and
// only the failure path probes MovieExists to tell "no such movie" (404) from
// a real error (500). Same shape, and the same reasoning, as the playlist add
// path in movie_playlist_handler.go. The read and delete paths keep their
// pre-check: neither can trip a foreign key, so nothing else would produce the
// documented 404.
func (app *Application) rejectWatchProgressWrite(w http.ResponseWriter, r *http.Request, writeErr error, movieID int64, logMessage, userMessage string) {
	exists, existsErr := app.Queries.MovieExists(r.Context(), movieID)
	if existsErr == nil && !exists {
		helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
		return
	}

	app.Logger.Error(logMessage, "error", writeErr, "movie_id", movieID)
	helpers.ErrorJSON(w, errors.New(userMessage))
}

func emptyMovieWatchProgressResponse() movieWatchProgressResponse {
	return movieWatchProgressResponse{
		ProgressSec: nil,
		DurationSec: nil,
		Watched:     false,
		UpdatedAt:   nil,
	}
}

func movieWatchProgressToResponse(row database.MovieWatchProgress) movieWatchProgressResponse {
	progressSec := row.ProgressSec
	durationSec := row.DurationSec
	updatedAt := row.UpdatedAt

	return movieWatchProgressResponse{
		ProgressSec: &progressSec,
		DurationSec: &durationSec,
		Watched:     row.Watched,
		UpdatedAt:   &updatedAt,
	}
}

func (app *Application) GetMovieWatchProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	movieID, err := parseMovieID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	if err := app.ensureMovieExists(r, movieID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to verify movie exists for watch progress", "error", err, "movie_id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to fetch watch progress"))
		return
	}

	row, err := app.Queries.GetMovieWatchProgress(r.Context(), database.GetMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
				Error: false,
				Data:  emptyMovieWatchProgressResponse(),
			})
			return
		}
		app.Logger.Error("failed to get movie watch progress", "error", err, "movie_id", movieID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch watch progress"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  movieWatchProgressToResponse(row),
	})
}

func (app *Application) GetContinueWatchingMovies(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	movies, err := app.Queries.GetContinueWatchingMovies(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to get continue watching movies", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to fetch movies"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movies": movies,
		},
	})
}

func (app *Application) UpdateMovieWatchProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	movieID, err := parseMovieID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	var req updateMovieWatchProgressRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.ProgressSec == nil {
		helpers.ErrorJSON(w, errors.New("progress_sec is required"), http.StatusBadRequest)
		return
	}
	if req.DurationSec == nil {
		helpers.ErrorJSON(w, errors.New("duration_sec is required"), http.StatusBadRequest)
		return
	}
	if req.SaveSessionID == nil {
		helpers.ErrorJSON(w, errors.New("save_session_id is required"), http.StatusBadRequest)
		return
	}
	if !validUUID(*req.SaveSessionID) {
		helpers.ErrorJSON(w, errors.New("save_session_id must be a valid UUID"), http.StatusBadRequest)
		return
	}
	if req.SaveSequence == nil {
		helpers.ErrorJSON(w, errors.New("save_sequence is required"), http.StatusBadRequest)
		return
	}
	if *req.SaveSequence <= 0 {
		helpers.ErrorJSON(w, errors.New("save_sequence must be greater than 0"), http.StatusBadRequest)
		return
	}

	progressVal := *req.ProgressSec
	durationVal := *req.DurationSec

	if math.IsNaN(progressVal) || math.IsNaN(durationVal) || math.IsInf(progressVal, 0) || math.IsInf(durationVal, 0) {
		helpers.ErrorJSON(w, errors.New("progress and duration must be finite numbers"), http.StatusBadRequest)
		return
	}
	if durationVal <= 0 {
		helpers.ErrorJSON(w, errors.New("duration_sec must be greater than 0"), http.StatusBadRequest)
		return
	}

	progressSec := helpers.ClampFloat64(progressVal, 0, durationVal)
	durationSec := durationVal

	if progressSec/durationSec >= watchCompletionThreshold {
		err := app.Queries.MarkMovieWatchedFromProgress(r.Context(), database.MarkMovieWatchedFromProgressParams{
			UserID:        userID,
			MovieID:       movieID,
			SaveSessionID: *req.SaveSessionID,
			SaveSequence:  *req.SaveSequence,
		})
		if err != nil {
			app.rejectWatchProgressWrite(w, r, err, movieID,
				"failed to mark movie watched from progress update", "failed to update watch progress")
			return
		}

		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: map[string]any{
				"watched": true,
			},
		})
		return
	}

	err = app.Queries.UpsertMovieWatchProgress(r.Context(), database.UpsertMovieWatchProgressParams{
		UserID:        userID,
		MovieID:       movieID,
		ProgressSec:   progressSec,
		DurationSec:   durationSec,
		SaveSessionID: *req.SaveSessionID,
		SaveSequence:  *req.SaveSequence,
	})
	if err != nil {
		app.rejectWatchProgressWrite(w, r, err, movieID,
			"failed to upsert movie watch progress", "failed to update watch progress")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"watched": false,
		},
	})
}

func (app *Application) DeleteMovieWatchProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	movieID, err := parseMovieID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	if err := app.ensureMovieExists(r, movieID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("movie not found"), http.StatusNotFound)
			return
		}
		app.Logger.Error("failed to verify movie exists for watch progress delete", "error", err, "movie_id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to clear watch progress"))
		return
	}

	if err := app.Queries.DeleteMovieWatchProgress(r.Context(), database.DeleteMovieWatchProgressParams{
		UserID:  userID,
		MovieID: movieID,
	}); err != nil {
		app.Logger.Error("failed to delete movie watch progress", "error", err, "movie_id", movieID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to clear watch progress"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"cleared": true,
		},
	})
}

func (app *Application) SetMovieWatched(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	movieID, err := parseMovieID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	var req setMovieWatchedRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.Watched == nil {
		helpers.ErrorJSON(w, errors.New("watched is required"), http.StatusBadRequest)
		return
	}

	watched := *req.Watched

	if watched {
		if err := app.Queries.MarkMovieWatched(r.Context(), database.MarkMovieWatchedParams{
			UserID:  userID,
			MovieID: movieID,
		}); err != nil {
			app.rejectWatchProgressWrite(w, r, err, movieID,
				"failed to mark movie watched", "failed to update watched status")
			return
		}
	} else {
		if err := app.Queries.MarkMovieUnwatched(r.Context(), database.MarkMovieUnwatchedParams{
			UserID:  userID,
			MovieID: movieID,
		}); err != nil {
			app.rejectWatchProgressWrite(w, r, err, movieID,
				"failed to mark movie unwatched", "failed to update watched status")
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movie_id": movieID,
			"watched":  watched,
		},
	})
}
