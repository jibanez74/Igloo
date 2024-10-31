package main

import (
	"errors"
	"fmt"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"log"
	"net/http"
	"strconv"
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

	app.session.Put(r.Context(), "user_id", user.ID)
	app.session.Put(r.Context(), "is_admin", user.IsAdmin)

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"thumb":    user.Thumb,
			"isAdmin":  user.IsAdmin,
		},
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

func (app *config) GetAuthUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(app.session.Get(r.Context(), "user_id"), 10, 64)
	if err != nil {
		helpers.ErrorJSON(w, err)
		return
	}

	log.Println(fmt.Sprintf("your id is %d", id))

	var user models.User
	user.ID = uint(id)

	status, err := app.repo.GetUserByID(&user)
	if err != nil {
		helpers.ErrorJSON(w, err, status)
		return
	}

	if !user.IsActive {
		helpers.ErrorJSON(w, errors.New("not authorized"), http.StatusUnauthorized)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"thumb":    user.Thumb,
			"isAdmin":  user.IsAdmin,
		},
	})
}
