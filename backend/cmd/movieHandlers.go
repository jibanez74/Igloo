package main

import (
	"errors"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"igloo/cmd/repository"
	"net/http"
	"os"
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

func (app *config) DirectStreamMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	var movie models.Movie
	movie.ID = uint(id)

	status, err := app.repo.GetMovieByID(&movie)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	status, err = helpers.CheckFileExist(movie.FilePath)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	file, err := os.Open(movie.FilePath)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", movie.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(movie.Size, 10))
	w.Header().Set("Content-Disposition", "inline; filename="+movie.Title)
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, movie.FilePath, movie.CreatedAt, file)

}

func (app *config) GetAllMovies(w http.ResponseWriter, r *http.Request) {
	var movies []repository.SimpleMovie

	status, err := app.repo.GetAllMovies(&movies)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	helpers.WriteJSON(w, status, map[string]any{
		"movies": movies,
		"count":  len(movies),
	})

}
