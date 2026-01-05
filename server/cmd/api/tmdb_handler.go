package main

import (
	"errors"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// GetMoviesInTheaters returns the latest movies currently playing in theaters.
// The response is limited to a maximum of 12 movies.
func (app *Application) GetMoviesInTheaters(w http.ResponseWriter, r *http.Request) {
	movies, err := app.Tmdb.GetMoviesInTheaters()
	if err != nil {
		app.Logger.Error("failed to get movies in theaters", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch movies in theaters"))
		return
	}

	const maxMovies = 12
	if len(movies) > maxMovies {
		movies = movies[:maxMovies]
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movies": movies,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// GetMovieByTmdbID returns a single movie from TMDB by its TMDB ID.
func (app *Application) GetMovieByTmdbID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid tmdb movie id"), http.StatusBadRequest)
		return
	}

	movie := &tmdb.TmdbMovie{TmdbID: id}
	err = app.Tmdb.GetTmdbMovieByID(movie)
	if err != nil {
		app.Logger.Error("failed to get movie from tmdb", "error", err, "tmdb_id", id)
		helpers.ErrorJSON(w, errors.New("failed to fetch movie from tmdb"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"movie": movie,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
