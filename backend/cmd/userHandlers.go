package main

import (
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"net/http"
)

func (app *config) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user models.User

	err := helpers.ReadJSON(w, r, &user)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	status, err := app.repo.CreateUser(&user)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	helpers.WriteJSON(w, status, map[string]any{
		"message": "User created successfully",
	})
}
