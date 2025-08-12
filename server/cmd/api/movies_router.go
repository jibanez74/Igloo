package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (app *Application) getTotalMovieCount(w http.ResponseWriter, r *http.Request) {
	count, err := app.Queries.GetMovieCount(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, map[string]int64{
		"count": count,
	})
}

func (app *Application) getLatestMovies(w http.ResponseWriter, r *http.Request) {
	movies, err := app.Queries.GetLatestMovies(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"movies": movies,
	})
}

func (app *Application) getMovieDetails(w http.ResponseWriter, r *http.Request) {
	movieID := chi.URLParam(r, "id")
	if movieID == "" {
		helpers.ErrorJSON(w, errors.New("movie id not in url params"), http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(movieID)
	if err != nil {
		helpers.ErrorJSON(w, fmt.Errorf("unable to parse id %s", movieID))
		return
	}

	movie, err := app.Queries.GetMovieDetails(context.Background(), int32(id))
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"movie": movie,
	})
}

func (app *Application) getMoviesPaginated(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 24
	}

	offset := (page - 1) * limit

	movies, err := app.Queries.GetMoviesPaginated(context.Background(), database.GetMoviesPaginatedParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})

	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	count, err := app.Queries.GetMovieCount(context.Background())
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	totalPages := (count + int64(limit) - 1) / int64(limit)

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"items":        movies,
		"current_page": page,
		"total_pages":  totalPages,
		"count":        count,
	})
}
