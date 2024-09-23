package main

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
)

func (app *config) FindOrCreateStudio(w http.ResponseWriter, r *http.Request) {
	var studio models.Studio

	err := helpers.ReadJSON(w, r, &studio)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	err = app.DB.Where("name = ?", studio.Name).First(&studio).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Studio found",
		Data:    studio,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
