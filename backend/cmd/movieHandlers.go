package main

import (
	"errors"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"igloo/cmd/repository"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (app *config) GetLatestMovies(w http.ResponseWriter, r *http.Request) {
	var movies []repository.SimpleMovie

	status, err := app.repo.GetLatestMovies(&movies)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	helpers.WriteJSON(w, status, map[string]any{
		"movies": movies,
	})
}

func (app *config) GetMoviesWithPagination(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	count, err := app.repo.GetMovieCount()
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	offset := (page - 1) * limit

	var movies []repository.SimpleMovie

	status, err := app.repo.GetMoviesWithPagination(&movies, limit, offset)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	pages := count / int64(limit)

	helpers.WriteJSON(w, status, map[string]any{
		"movies": movies,
		"pages":  pages,
		"page":   page,
		"count":  count,
	})
}

func (app *config) GetMovieByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
	}

	var movie models.Movie
	movie.ID = uint(id)

	status, err := app.repo.GetMovieByID(&movie)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	helpers.WriteJSON(w, status, map[string]any{
		"movie": movie,
	})
}

func (app *config) CreateMovie(w http.ResponseWriter, r *http.Request) {
	var movie models.Movie

	err := helpers.ReadJSON(w, r, &movie)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	if movie.FilePath == "" {
		helpers.ErrorJSON(w, errors.New("file path is required"))
		return
	}

	if movie.TmdbID != "" {
		err = app.tmdb.GetTmdbMovieByID(&movie)
		if err != nil {
			helpers.ErrorJSON(w, err, http.StatusInternalServerError)
			return
		}
	}

	err = helpers.GetMovieMetadata(&movie)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusInternalServerError)
		return
	}

	status, err := app.repo.CreateMovie(&movie)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	helpers.WriteJSON(w, status, map[string]any{
		"movie": movie,
	})
}
