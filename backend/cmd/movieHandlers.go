package main

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (app *config) GetLatestMovies(w http.ResponseWriter, r *http.Request) {
	var movies [12]models.Movie

	err := app.DB.Order("created_at desc").Limit(12).Find(&movies).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "latest movies were returned successfully",
		Data:    movies,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *config) GetMovieByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusInternalServerError)
		return
	}

	var movie models.Movie

	err = app.DB.First(&movie, uint(id)).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "movie was returned successfully",
		Data:    movie,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *config) GetMovies(w http.ResponseWriter, r *http.Request) {
	// get all movies in alphabetical order
	var movies []models.Movie

	err := app.DB.Order("title").Find(&movies).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "movies were returned successfully",
		Data:    movies,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *config) CreateMovie(w http.ResponseWriter, r *http.Request) {
	var movie models.Movie

	err := helpers.ReadJSON(w, r, &movie)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = app.DB.Create(&movie).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "movie was created successfully",
		Data:    movie,
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}
