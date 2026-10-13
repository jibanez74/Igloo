package main

import (
	"igloo/cmd/internal/helpers"
	"net/http"
)

func (app *Application) HealthCheck(w http.ResponseWriter, r *http.Request) {
	err := app.DB.PingContext(r.Context())
	if err != nil {
		app.serverError(w, err, "health check database ping failed", internalServerErrorMessage)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "server is healthy"})
}
