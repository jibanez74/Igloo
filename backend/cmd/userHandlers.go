package main

import (
	"errors"
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// create a new user by an admin
func (app *config) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user models.User

	err := helpers.ReadJSON(w, r, &user)
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to read request body")
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	err = app.DB.Create(&user).Error
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to create user")
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "user created",
	}

	helpers.WriteJSON(w, http.StatusCreated, res)
}

// allow an admin to fetch a user by id
func (app *config) GetUserByID(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		msg := "user id is required"
		app.ErrorLog.Print(msg)
		helpers.ErrorJSON(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to parse id to uint")
		helpers.ErrorJSON(w, err)
		return
	}

	var user models.User

	err = app.DB.First(&user, uint(id)).Error
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to find user")
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "user found",
		Data:    user,
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *config) GetAuthUser(w http.ResponseWriter, r *http.Request) {
	id := app.Session.GetInt(r.Context(), "user_id")

	var user models.User

	err := app.DB.First(&user, uint(id)).Error
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
			"thumb":    user.Thumb,
			"isAdmin":  user.IsAdmin,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// allows an admin to fetch a list of all users with pagination
func (app *config) GetUsersWithPagination(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "10"
	}

	p, err := strconv.Atoi(page)
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to parse page to int")
		helpers.ErrorJSON(w, err)
		return
	}

	l, err := strconv.Atoi(limit)
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to parse limit to int")
		helpers.ErrorJSON(w, err)
		return
	}

	var count int64

	err = app.DB.Model(&models.User{}).Count(&count).Error
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to count users")
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	var users []struct {
		ID       uint
		Name     string `json:"name"`
		Email    string `json:"email"`
		IsAdmin  bool   `json:"isAdmin"`
		IsActive bool   `json:"isActive"`
	}

	err = app.DB.Model(&models.User{}).Select("id, name, email, is_admin, is_active").Offset((p - 1) * l).Limit(l).Find(&users).Error
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to fetch users")
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	pages := int(math.Ceil(float64(count) / float64(l)))

	res := helpers.JSONResponse{
		Error:   false,
		Message: "users fetched",
		Data: map[string]any{
			"users": users,
			"count": count,
			"pages": pages,
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *config) DeleteUserByID(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		msg := "user id is required"
		app.ErrorLog.Print(msg)
		helpers.ErrorJSON(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to parse id to uint")
		helpers.ErrorJSON(w, err)
		return
	}

	err = app.DB.Delete(&models.User{}, uint(id)).Error
	if err != nil {
		app.ErrorLog.Print(err)
		app.InfoLog.Print("unable to delete user")
		helpers.ErrorJSON(w, err, helpers.GormStatusCode(err))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "user deleted",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
