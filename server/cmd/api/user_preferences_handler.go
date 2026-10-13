package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/preroll"
)

// Account-scoped playback preferences. Unlike the device preferences the web
// client keeps in localStorage, these follow the account across browsers and
// devices, and the TV client reads the same values.

type trailerPreferencesRequest struct {
	Enabled *bool   `json:"enabled"`
	Count   *int    `json:"count"`
	Source  *string `json:"source"`
}

type trailerPreferencesResponse struct {
	Enabled bool   `json:"enabled"`
	Count   int    `json:"count"`
	Source  string `json:"source"`
}

// loadTrailerPreferences returns the stored preferences, or the defaults when
// the user has never saved any.
func (app *Application) loadTrailerPreferences(ctx context.Context, userID int64) (preroll.Preferences, error) {
	row, err := app.Queries.GetUserPreferences(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return preroll.DefaultPreferences(), nil
		}
		return preroll.Preferences{}, err
	}
	return preroll.Preferences{
		Enabled: row.TrailersEnabled,
		Count:   int(row.TrailersCount),
		Source:  row.TrailersSource,
	}, nil
}

func (app *Application) GetTrailerPreferences(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	prefs, err := app.loadTrailerPreferences(r.Context(), userID)
	if err != nil {
		app.serverError(w, err, "failed to load trailer preferences", internalServerErrorMessage, "user_id", userID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: trailerPreferencesResponse(prefs)})
}

func (app *Application) UpdateTrailerPreferences(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	var req trailerPreferencesRequest
	err := helpers.ReadJSON(w, r, &req)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}
	missingField := req.Enabled == nil || req.Count == nil || req.Source == nil
	if missingField {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	prefs := preroll.Preferences{Enabled: *req.Enabled, Count: *req.Count, Source: *req.Source}
	err = preroll.ValidatePreferences(prefs)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	row, err := app.Queries.UpsertUserPreferences(r.Context(), database.UpsertUserPreferencesParams{
		UserID:          userID,
		TrailersEnabled: prefs.Enabled,
		TrailersCount:   int64(prefs.Count),
		TrailersSource:  prefs.Source,
	})
	if err != nil {
		app.serverError(w, err, "failed to save trailer preferences", internalServerErrorMessage, "user_id", userID)
		return
	}

	stored := preroll.Preferences{Enabled: row.TrailersEnabled, Count: int(row.TrailersCount), Source: row.TrailersSource}
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: trailerPreferencesResponse(stored)})
}
