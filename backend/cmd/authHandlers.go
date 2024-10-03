package main

import (
	"errors"
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"net/http"
)

func (app *config) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	err := helpers.ReadJSON(w, r, &req)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		helpers.ErrorJSON(w, errors.New("missing required fields"))
		return
	}

	var user models.User

	err = app.DB.Where("email = ? AND username = ?", req.Email, req.Username).First(&user).Error
	if err != nil {
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	match, err := user.PasswordMatches(req.Password)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New("invlida password"), http.StatusUnauthorized)
		return
	}

	app.Session.Put(r.Context(), "user_id", user.ID)
	app.Session.Put(r.Context(), "is_admin", user.IsAdmin)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Login successful",
		Data: map[string]any{
			"ID":       user.ID,
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"thumb":    user.Thumb,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *config) Logout(w http.ResponseWriter, r *http.Request) {
	err := app.Session.Destroy(r.Context())
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Logout successful",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
