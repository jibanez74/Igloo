package main

import (
	"errors"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"log"
	"net/http"
)

func (app *config) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := helpers.ReadJSON(w, r, &req)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	var user models.User
	user.Username = req.Username
	user.Email = req.Email

	err = app.repo.GetAuthUser(&user)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid credentials"), http.StatusUnauthorized)
		return
	}

	match, err := user.PasswordMatches(req.Password)
	if err != nil {
		log.Println(err)
		helpers.ErrorJSON(w, err)
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New("invalid credentials"), http.StatusUnauthorized)
		return
	}

	token, err := helpers.GenerateLongLivedToken(&user, app.cookieSecret)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(helpers.LongLivedExp.Seconds()),
	})

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"isAdmin":  user.IsAdmin,
			"thumb":    user.Thumb,
		},
	})
}
