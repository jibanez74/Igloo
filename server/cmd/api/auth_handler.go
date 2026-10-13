package main

import (
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strings"
	"time"
)

const (
	cookieUserID              = "user_id"
	invalidCredentialsMessage = "invalid email or password provided"

	// Logged wherever a stored hash is compared: login, device login, and the
	// password change in user_handler.go.
	comparePasswordHashLogMessage = "failed to compare password hash"
)

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Browser and device logins accept the same credentials, so they draw on one
// per-IP budget; a limit on only one of them would leave the other open to
// password guessing.
const (
	loginAttemptLimit  = 10
	loginAttemptWindow = 5 * time.Minute
)

func (app *Application) allowLoginAttempt(r *http.Request) bool {
	return app.AuthLimiter.Allow("login:"+clientIP(r), loginAttemptLimit, loginAttemptWindow)
}

// verifyCredentials loads the user an email and password sign in as, answering
// 401 for an unknown email or a wrong password and 500 for a failed lookup.
func (app *Application) verifyCredentials(w http.ResponseWriter, r *http.Request, email, password string) (database.User, bool) {
	user, err := app.Queries.GetUserByEmail(r.Context(), email)
	if errors.Is(err, sql.ErrNoRows) {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusUnauthorized)
		return user, false
	}
	if err != nil {
		app.serverError(w, err, "failed to fetch user from database for login", internalServerErrorMessage)
		return user, false
	}

	match, err := helpers.PasswordMatches(password, user.Password)
	if err != nil {
		app.serverError(w, err, comparePasswordHashLogMessage, internalServerErrorMessage, "email", email)
		return user, false
	}
	if !match {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusUnauthorized)
		return user, false
	}

	return user, true
}

func (app *Application) AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	if !app.allowLoginAttempt(r) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	var request AuthRequest

	err := helpers.ReadJSON(w, r, &request)
	if err != nil {
		app.Logger.Error("failed to parse request body in login process", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to parse email and password from request body"), http.StatusBadRequest)
		return
	}

	if request.Email == "" || request.Password == "" {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusBadRequest)
		return
	}

	user, ok := app.verifyCredentials(w, r, request.Email, request.Password)
	if !ok {
		return
	}

	err = app.SessionManager.RenewToken(r.Context())
	if err != nil {
		app.serverError(w, err, "failed to renew session token", internalServerErrorMessage, "user", user.Name)
		return
	}

	app.SessionManager.Put(r.Context(), cookieUserID, user.ID)
	app.SessionManager.Put(r.Context(), cookieSessionVersion, user.SessionVersion)
	app.rememberSessionVersion(user.ID, user.SessionVersion)

	res := helpers.JSONResponse{
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
	if !app.allowLoginAttempt(r) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	var request DeviceAuthRequest

	err := helpers.ReadJSON(w, r, &request)
	if err != nil {
		app.Logger.Error("failed to parse request body in device login", "error", err)
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	request.DeviceName = strings.TrimSpace(request.DeviceName)
	if request.Email == "" || request.Password == "" {
		helpers.ErrorJSON(w, errors.New(invalidCredentialsMessage), http.StatusBadRequest)
		return
	}

	err = validateDeviceInfo(request.DeviceName, request.Platform, request.AppVersion)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	user, ok := app.verifyCredentials(w, r, request.Email, request.Password)
	if !ok {
		return
	}

	token, device, ok := app.issueDeviceToken(w, r, user.ID, request.DeviceName, request.Platform, request.AppVersion)
	if !ok {
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: fmt.Sprintf("Hello %s, welcome to your media library!", user.Name),
		Data: map[string]any{
			"token":  token,
			"device": deviceResponseMap(database.GetDevicesByUserRow(device)),
		},
	})
}

func (app *Application) GetCurrentAuthUser(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		app.userLookupFailed(w, err, "failed to fetch user from database", userID)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"user": newUserResponse(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
		},
	})
}

func (app *Application) DestroySession(w http.ResponseWriter, r *http.Request) {
	auth := deviceAuthFrom(r.Context())
	if auth != nil {
		_, err := app.Queries.DeleteDeviceForUser(r.Context(), database.DeleteDeviceForUserParams{
			ID:     auth.DeviceID,
			UserID: auth.UserID,
		})
		if err != nil {
			app.serverError(w, err, "failed to revoke device during logout", internalServerErrorMessage)
			return
		}

		app.forgetDevice(auth.DeviceID)
		app.Logger.Info("device revoked via logout", "user_id", auth.UserID, "device_id", auth.DeviceID)
	} else {
		err := app.SessionManager.Destroy(r.Context())
		if err != nil {
			app.serverError(w, err, "failed to destroy session during logout", internalServerErrorMessage)
			return
		}
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: "You have been logged out successfully"})
}
