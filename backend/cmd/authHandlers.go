package main

import (
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"net/http"
)

func (app *config) Login(w http.ResponseWriter, r *http.Request) {
	var user models.User

	err := helpers.ReadJSON(w, r, &user)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	status, err := app.repo.GetAuthUser(&user)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	app.session.Put(r.Context(), "user_id", user.ID)
	app.session.Put(r.Context(), "is_admin", user.IsAdmin)

	helpers.WriteJSON(w, status, map[string]any{
		"name":     user.Name,
		"email":    user.Email,
		"username": user.Username,
		"thumb":    user.Thumb,
		"isAdmin":  user.IsAdmin,
	})
}

func (app *config) Logout(w http.ResponseWriter, r *http.Request) {
	err := app.session.Destroy(r.Context())
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "logout successful",
	})
}
