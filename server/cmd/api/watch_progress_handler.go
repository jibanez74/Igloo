package main

import (
	"database/sql"
	"errors"
	"math"
	"net/http"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

type updateMovieWatchProgressRequest struct {
	ProgressSec float64 `json:"progress_sec"`
	DurationSec float64 `json:"duration_sec"`
}

type setMovieWatchedRequest struct {
	Watched bool `json:"watched"`
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

func (app *Application) requireSessionUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return 0, false
	}
	return userID, true
}

func (app *Application) ensureMovieExists(r *http.Request, movieID int64) error {
	_, err := app.Queries.GetMovieByID(r.Context(), movieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}
	return nil
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
	userID, ok := app.requireSessionUserID(w, r)
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

func (app *Application) UpdateMovieWatchProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
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
		app.Logger.Error("failed to verify movie exists for watch progress update", "error", err, "movie_id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to update watch progress"))
		return
	}

	var req updateMovieWatchProgressRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if math.IsNaN(req.ProgressSec) || math.IsNaN(req.DurationSec) || math.IsInf(req.ProgressSec, 0) || math.IsInf(req.DurationSec, 0) {
		helpers.ErrorJSON(w, errors.New("progress and duration must be finite numbers"), http.StatusBadRequest)
		return
	}
	if req.DurationSec <= 0 {
		helpers.ErrorJSON(w, errors.New("duration_sec must be greater than 0"), http.StatusBadRequest)
		return
	}

	progressSec := helpers.ClampFloat64(req.ProgressSec, 0, req.DurationSec)
	durationSec := req.DurationSec

	if progressSec/durationSec >= helpers.WATCH_COMPLETION_THRESHOLD {
		if err := app.Queries.MarkMovieWatched(r.Context(), database.MarkMovieWatchedParams{
			UserID:  userID,
			MovieID: movieID,
		}); err != nil {
			app.Logger.Error("failed to mark movie watched from progress update", "error", err, "movie_id", movieID, "user_id", userID)
			helpers.ErrorJSON(w, errors.New("failed to update watch progress"))
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

	if err := app.Queries.UpsertMovieWatchProgress(r.Context(), database.UpsertMovieWatchProgressParams{
		UserID:      userID,
		MovieID:     movieID,
		ProgressSec: progressSec,
		DurationSec: durationSec,
	}); err != nil {
		app.Logger.Error("failed to upsert movie watch progress", "error", err, "movie_id", movieID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to update watch progress"))
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
	userID, ok := app.requireSessionUserID(w, r)
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
	userID, ok := app.requireSessionUserID(w, r)
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
		app.Logger.Error("failed to verify movie exists for watched update", "error", err, "movie_id", movieID)
		helpers.ErrorJSON(w, errors.New("failed to update watched status"))
		return
	}

	var req setMovieWatchedRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if req.Watched {
		if err := app.Queries.MarkMovieWatched(r.Context(), database.MarkMovieWatchedParams{
			UserID:  userID,
			MovieID: movieID,
		}); err != nil {
			app.Logger.Error("failed to mark movie watched", "error", err, "movie_id", movieID, "user_id", userID)
			helpers.ErrorJSON(w, errors.New("failed to update watched status"))
			return
		}
	} else {
		if err := app.Queries.MarkMovieUnwatched(r.Context(), database.MarkMovieUnwatchedParams{
			UserID:  userID,
			MovieID: movieID,
		}); err != nil {
			app.Logger.Error("failed to mark movie unwatched", "error", err, "movie_id", movieID, "user_id", userID)
			helpers.ErrorJSON(w, errors.New("failed to update watched status"))
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movie_id": movieID,
			"watched":  req.Watched,
		},
	})
}
