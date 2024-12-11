package main

import (
	"errors"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"net/http"
	"strconv"
)

func (app *config) GetAuthenticatedUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := helpers.UserIDFromContext(r.Context())
	if !ok {
		helpers.ErrorJSON(w, errors.New("unauthorized"), http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	var user models.User
	user.ID = uint(id)

	status, err := app.repo.GetUserByID(&user)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	// Return user data
	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":       user.ID,
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"isAdmin":  user.IsAdmin,
			"thumb":    user.Thumb,
		},
	})
}

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
