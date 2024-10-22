package main

import (
	"errors"
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// get the latest movies in the system
func (app *config) GetLatestMovies(w http.ResponseWriter, r *http.Request) {
	var movies [12]models.SimpleMovie

	err := app.DB.Model(&models.Movie{}).Select("id, title, thumb, year").Order("created_at desc").Limit(12).Find(&movies).Error
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

// get a movie by its id
func (app *config) GetMovieByID(w http.ResponseWriter, r *http.Request) {
	movieID := chi.URLParam(r, "id")
	if movieID == "" {
		msg := "movie id is required"
		app.ErrorLog.Print(msg)
		helpers.ErrorJSON(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(movieID, 10, 64)
	if err != nil {
		app.ErrorLog.Print(err)
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	var movie models.Movie

	err = app.DB.Preload("CastList.Artist").Preload("CrewList.Artist").Preload("Genres").Preload("Studios").Preload("Trailers").Preload("VideoList").Preload("AudioList").Preload("SubtitleList").Preload("ChapterList").First(&movie, uint(id)).Error
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

// allows an admin to create a movie
func (app *config) CreateMovie(w http.ResponseWriter, r *http.Request) {
	var movie models.Movie

	err := helpers.ReadJSON(w, r, &movie)
	if err != nil {
		msg := "unable to read request body"
		app.ErrorLog.Print(msg)
		helpers.ErrorJSON(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	err = helpers.GetTmdbMovie(&movie, false)
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to get movie from tmdb")
	}

	err = app.DB.Create(&movie).Error
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to create movie")
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
