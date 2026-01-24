package main

import (
	"fmt"
	"igloo/cmd/internal/helpers"
	"net/http"
)

// HealthCheck returns the health status of the application.
// It pings the database to verify connectivity.
func (app *Application) HealthCheck(w http.ResponseWriter, r *http.Request) {
	err := app.DB.PingContext(r.Context())
	if err != nil {
		helpers.ErrorJSON(w, fmt.Errorf("fail to ping the data base\n%v", err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "server is healthy",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
