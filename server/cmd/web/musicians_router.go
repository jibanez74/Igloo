package main

import (
	"context"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"

	"github.com/jackc/pgx/v5"
)

func (app *Application) RouteGetMusicianCount(w http.ResponseWriter, r *http.Request) {
	count, err := app.Queries.GetMusicianCount(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to get musician count"))
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

func (app *Application) RouteGetMusicianList(w http.ResponseWriter, r *http.Request) {
	musicians, err := app.Queries.GetMusicianList(context.Background())
	if err != nil {
		if err == pgx.ErrNoRows {
			helpers.ErrorJSON(w, errors.New("no musicians were found"), http.StatusNotFound)
		} else {
			helpers.ErrorJSON(w, errors.New("internal server error while fetching musician list"))
		}

		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"musicians": musicians,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) RouteCreateMusician(w http.ResponseWriter, r *http.Request) {
	var createMusician database.CreateMusicianParams

	err := helpers.ReadJSON(w, r, &createMusician)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to read request body to create musician"))
		return
	}

	musician, err := app.Queries.CreateMusician(context.Background(), createMusician)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("fail to create musician in data base"))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "musician created successfully",
		Data: map[string]any{
			"musician": musician,
		},
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}
