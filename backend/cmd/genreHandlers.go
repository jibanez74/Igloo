package main

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
)

func (app *config) FIndOrCreateGenre(w http.ResponseWriter, r *http.Request) {
	var genre models.Genre

	err := helpers.ReadJSON(w, r, &genre)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = app.DB.Where("tag = ?", genre.Tag).First(&genre).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Genre found",
		Data:    genre,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
