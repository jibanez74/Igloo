package main

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

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

func (app *config) GetMovieByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusInternalServerError)
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

func (app *config) GetMovies(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 24
	}

	offset := (page - 1) * pageSize

	var totalCount int64

	err = app.DB.Model(&models.Movie{}).Count(&totalCount).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	var movies []models.SimpleMovie

	err = app.DB.Model(&models.Movie{}).Select("id, title, thumb, year").Order("title").Offset(offset).Limit(pageSize).Find(&movies).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(pageSize)))

	res := helpers.JSONResponse{
		Error:   false,
		Message: "movies were returned successfully",
		Data: map[string]any{
			"movies":      movies,
			"currentPage": page,
			"pageSize":    pageSize,
			"totalPages":  totalPages,
			"totalCount":  totalCount,
		},
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
