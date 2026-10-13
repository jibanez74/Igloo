package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
)

func (app *Application) HealthCheck(w http.ResponseWriter, r *http.Request) {
	err := app.DB.PingContext(r.Context())
	if err != nil {
		app.Logger.Error("health check database ping failed", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "server is healthy"})
}
