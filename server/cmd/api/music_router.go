package main

import (
	"net/http"

	"igloo/cmd/internal/helpers"
)

func (app *Application) getAlbumsCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	count, err := app.Queries.GetAlbumsCount(ctx)
	if err != nil {
		app.Logger.Error("Failed to get albums count: " + err.Error())
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]int64{
			"count": count,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
