package main

import (
	"context"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

	err := helpers.ReadJSON(w, r, &createMusician, 1024*1024)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body format"), http.StatusBadRequest)
		return
	}

	musician, err := app.Queries.CreateMusician(context.Background(), createMusician)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				if strings.Contains(pgErr.ConstraintName, "musicians_spotify_id") {
					helpers.ErrorJSON(w, errors.New("a musician with this Spotify ID already exists"), http.StatusConflict)
				} else {
					helpers.ErrorJSON(w, errors.New("a musician with this information already exists"), http.StatusConflict)
				}
				return
			case "23502":
				helpers.ErrorJSON(w, errors.New("required fields cannot be empty"), http.StatusBadRequest)
				return
			case "23514":
				helpers.ErrorJSON(w, errors.New("invalid data provided: check constraints failed"), http.StatusBadRequest)
				return
			default:
				helpers.ErrorJSON(w, errors.New("database error occurred while creating musician"), http.StatusInternalServerError)
				return
			}
		}

		helpers.ErrorJSON(w, errors.New("failed to create musician"), http.StatusInternalServerError)
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
