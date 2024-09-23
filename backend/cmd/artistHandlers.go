package main

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
)

func (app *config) FindOrCreateArtist(w http.ResponseWriter, r *http.Request) {
	var artist models.Artist

	err := helpers.ReadJSON(w, r, &artist)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = app.DB.Where("name = ?", artist.Name).FirstOrCreate(&artist).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "artists was returned successfully",
		Data:    artist,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
