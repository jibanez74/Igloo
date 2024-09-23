package main

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
)

func (app *config) FindOrCreateArtists(w http.ResponseWriter, r *http.Request) {
	var artists []models.Artist

	err := helpers.ReadJSON(w, r, &artists)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	tx := app.DB.Begin()

	for _, artist := range artists {
		err = tx.Where("name = ?", artist.Name).FirstOrCreate(&artist).Error
		if err != nil {
			tx.Rollback()
			helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
			return
		}

		artists = append(artists, artist)
	}

	tx.Commit()

	res := helpers.JSONResponse{
		Error:   false,
		Message: "artists was returned successfully",
		Data:    artists,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
