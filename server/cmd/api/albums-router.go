package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/helpers"
	"net/http"
)

func (app *Application) RouteGetLatestAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := app.Queries.GetLatestAlbums(context.Background())
	if err != nil {
		app.Logger.Error(fmt.Sprintf("fail to fetch latest albums from database\n%s", err.Error()))
		helpers.ErrorJSON(w, errors.New(INTERNAL_SERVER_ERROR))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"albums": albums,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
