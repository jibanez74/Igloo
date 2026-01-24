package main

import (
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/helpers"
	"net/http"
)

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RouteLogin authenticates a user with email and password.
func (app *Application) AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	var request AuthRequest

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		app.Logger.Error("failed to parse request body in login process", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to parse email and password from request body"), http.StatusBadRequest)
		return
	}

	if request.Email == "" || request.Password == "" {
		helpers.ErrorJSON(w, errors.New(helpers.INVALID_CREDENTIALS_MESSAGE), http.StatusBadRequest)
		return
	}

	user, err := app.Queries.GetUserByEmail(r.Context(), request.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(helpers.INVALID_CREDENTIALS_MESSAGE), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user from database for login", "error", err)
			helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		}

		return
	}

	match, err := helpers.PasswordMatches(request.Password, user.Password)
	if err != nil {
		app.Logger.Error("failed to compare password hash", "error", err, "email", request.Email)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New(helpers.INVALID_CREDENTIALS_MESSAGE), http.StatusUnauthorized)
		return
	}

	err = app.SessionManager.RenewToken(r.Context())
	if err != nil {
		app.Logger.Error("failed to renew session token", "error", err, "user", user.Name)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, user.ID)

	res := helpers.JSONResponse{
		Error:   false,
		Message: fmt.Sprintf("Hello %s, welcome to your media library!", user.Name),
	}

	app.Logger.Info("user logged in successfully", "user", user.Name, "id", user.ID)

	helpers.WriteJSON(w, http.StatusOK, res)
}

// Gets the current user's profile from db using the id stored in the session
func (app *Application) GetCurrentAuthUser(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user from database", "error", err, "id", userID)
			helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		}

		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": map[string]any{
				"id":         user.ID,
				"name":       user.Name,
				"email":      user.Email,
				"is_admin":   user.IsAdmin,
				"avatar":     user.Avatar,
				"created_at": user.CreatedAt,
				"updated_at": user.UpdatedAt,
			},
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

// Destroys the current session and logs out the user
func (app *Application) DestroySession(w http.ResponseWriter, r *http.Request) {
	err := app.SessionManager.Destroy(r.Context())
	if err != nil {
		app.Logger.Error("failed to destroy session during logout", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "You have been logged out successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
