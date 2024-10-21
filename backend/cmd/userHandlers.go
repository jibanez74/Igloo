package main

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
)

func (app *config) GetAuthUser(w http.ResponseWriter, r *http.Request) {
	id := app.Session.GetInt(r.Context(), "user_id")

	var user models.User

	err := app.DB.Model(&models.User{}).First(&user, uint(id)).Error
	if err != nil {
		app.ErrorLog.Print(err)
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "user fetched",
		Data: map[string]any{
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"isAdmin":  user.IsAdmin,
			"thumb":    user.Thumb,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
