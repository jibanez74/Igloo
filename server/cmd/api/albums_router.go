package main

import (
	"context"
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (app *Application) RouteGetAlbumCount(w http.ResponseWriter, r *http.Request) {
	count, err := app.Queries.GetAlbumCount(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to get album count"))
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

func (app *Application) RouteGetLatestAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := app.Queries.GetLatestAlbums(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to get latest albums from the data base"))
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

func (app *Application) RouteGetAlbumDetails(w http.ResponseWriter, r *http.Request) {
	albumIDStr := chi.URLParam(r, "id")
	if albumIDStr == "" {
		helpers.ErrorJSON(w, errors.New("album ID is required"), http.StatusBadRequest)
		return
	}

	albumID, err := strconv.Atoi(albumIDStr)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid album ID format"))
		return
	}

	albumDetails, err := app.Queries.GetAlbumDetails(context.Background(), int32(albumID))
	if err != nil {
		if err == pgx.ErrNoRows {
			helpers.ErrorJSON(w, errors.New("album not found"), http.StatusNotFound)
		} else {
			helpers.ErrorJSON(w, errors.New("fail to get album details"))
		}

		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"album": albumDetails,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
