package main

import (
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	cookieUserID              = "user_id"
	invalidCredentialsMessage = "invalid email or password provided"
)

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (app *Application) AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	var request AuthRequest

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		app.Logger.Error("failed to parse request body in login process", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to parse email and password from request body"), http.StatusBadRequest)
		return
	}

	if request.Email == "" || request.Password == "" {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusBadRequest)
		return
	}

	user, err := app.Queries.GetUserByEmail(r.Context(), request.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user from database for login", "error", err)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		}

		return
	}

	match, err := helpers.PasswordMatches(request.Password, user.Password)
	if err != nil {
		app.Logger.Error("failed to compare password hash", "error", err, "email", request.Email)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusUnauthorized)
		return
	}

	err = app.SessionManager.RenewToken(r.Context())
	if err != nil {
		app.Logger.Error("failed to renew session token", "error", err, "user", user.Name)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	app.SessionManager.Put(r.Context(), cookieUserID, user.ID)

	res := helpers.JSONResponse{
		Error:   false,
		Message: fmt.Sprintf("Hello %s, welcome to your media library!", user.Name),
	}

	app.Logger.Info("user logged in successfully", "user", user.Name, "id", user.ID)

	helpers.WriteJSON(w, http.StatusOK, res)
}

type DeviceAuthRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

// AuthenticateDevice is the password-based login path for TV / mobile
// clients. It issues a long-lived bearer token and never touches the session.
func (app *Application) AuthenticateDevice(w http.ResponseWriter, r *http.Request) {
	if !app.AuthLimiter.Allow("dlogin:"+clientIP(r), 10, 5*time.Minute) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	var request DeviceAuthRequest

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		app.Logger.Error("failed to parse request body in device login", "error", err)
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	request.DeviceName = strings.TrimSpace(request.DeviceName)
	if request.Email == "" || request.Password == "" {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusBadRequest)
		return
	}

	if request.DeviceName == "" || len(request.DeviceName) > maxDeviceNameLength {
		helpers.ErrorJSON(w, errors.New("device_name is required and must be at most 100 characters"), http.StatusBadRequest)
		return
	}

	user, err := app.Queries.GetUserByEmail(r.Context(), request.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user from database for device login", "error", err)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		}

		return
	}

	match, err := helpers.PasswordMatches(request.Password, user.Password)
	if err != nil {
		app.Logger.Error("failed to compare password hash", "error", err, "email", request.Email)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	if !match {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusUnauthorized)
		return
	}

	token, device, ok := app.issueDeviceToken(w, r, user.ID, request.DeviceName, request.Platform, request.AppVersion)
	if !ok {
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: fmt.Sprintf("Hello %s, welcome to your media library!", user.Name),
		Data: map[string]any{
			"token":  token,
			"device": deviceResponseMap(device.ID, device.Name, device.Platform, device.AppVersion, device.CreatedAt, device.LastUsedAt, true),
		},
	})
}

func (app *Application) GetCurrentAuthUser(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user from database", "error", err, "id", userID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		}

		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin, user.CreatedAt, user.UpdatedAt),
		},
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) DestroySession(w http.ResponseWriter, r *http.Request) {
	if auth := deviceAuthFrom(r.Context()); auth != nil {
		_, err := app.Queries.DeleteDeviceForUser(r.Context(), database.DeleteDeviceForUserParams{
			ID:     auth.DeviceID,
			UserID: auth.UserID,
		})
		if err != nil {
			app.Logger.Error("failed to revoke device during logout", "error", err)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}

		app.DeviceLastSeen.Delete(strconv.FormatInt(auth.DeviceID, 10))

		app.Logger.Info("device revoked via logout", "user_id", auth.UserID, "device_id", auth.DeviceID)

		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error:   false,
			Message: "You have been logged out successfully",
		})

		return
	}

	err := app.SessionManager.Destroy(r.Context())
	if err != nil {
		app.Logger.Error("failed to destroy session during logout", "error", err)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	res := helpers.JSONResponse{
		Error:   false,
		Message: "You have been logged out successfully",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
