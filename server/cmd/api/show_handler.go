package main

import (
	"errors"
	"net/http"

	"igloo/cmd/internal/helpers"
)

func (app *Application) GetLatestShows(w http.ResponseWriter, r *http.Request) {
	shows, err := app.Queries.GetLatestShows(r.Context())
	if err != nil {
		app.Logger.Error("failed to get latest shows", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch latest shows"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"shows": shows},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
