package main

import (
	"context"
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
)

func (app *Application) RouteGetTrackCount(w http.ResponseWriter, r *http.Request) {
	count, err := app.Queries.GetTrackCount(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to get track count"))
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
