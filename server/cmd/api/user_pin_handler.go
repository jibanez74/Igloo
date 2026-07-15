package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

const (
	userPinLength              = 4
	pinAttemptLimit            = 5
	pinAttemptWindow           = time.Minute
	currentPinIncorrectMessage = "current PIN is incorrect"
)

// validatePin enforces exactly 4 ASCII digits. Byte-wise comparison rejects
// non-ASCII digit runes that a rune-based check would accept.
func validatePin(pin string) error {
	if len(pin) != userPinLength {
		return errors.New("pin must be exactly 4 digits")
	}
	for i := 0; i < len(pin); i++ {
		if pin[i] < '0' || pin[i] > '9' {
			return errors.New("pin must be exactly 4 digits")
		}
	}
	return nil
}

// allowPinAttempt rate-limits every code path that compares a stored PIN. The
// 4-digit keyspace is small, so guesses through verify and change share one
// per-user bucket.
func (app *Application) allowPinAttempt(userID int64) bool {
	return app.AuthLimiter.Allow("pin:"+strconv.FormatInt(userID, 10), pinAttemptLimit, pinAttemptWindow)
}

type UpdateUserPinRequest struct {
	Pin        string `json:"pin"`
	CurrentPin string `json:"current_pin"`
}

// UpdateUserPin sets, changes, or removes the profile PIN. An empty pin
// removes it (mirroring the avatar convention). Device tokens are allowed so
// a TV client can set the PIN on first sign-in, but changing or removing an
// existing PIN requires the current PIN.
func (app *Application) UpdateUserPin(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	var req UpdateUserPinRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	removing := req.Pin == ""
	if !removing {
		if err := validatePin(req.Pin); err != nil {
			helpers.ErrorJSON(w, err, http.StatusBadRequest)
			return
		}
	}

	currentUser, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user for pin update", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		}
		return
	}

	if currentUser.Pin.Valid {
		if !app.allowPinAttempt(userID) {
			helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
			return
		}

		if req.CurrentPin == "" {
			helpers.ErrorJSON(w, errors.New("current PIN is required"), http.StatusBadRequest)
			return
		}

		if req.CurrentPin != currentUser.Pin.String {
			helpers.ErrorJSON(w, errors.New(currentPinIncorrectMessage), http.StatusUnauthorized)
			return
		}
	}

	pinValue := sql.NullString{String: req.Pin, Valid: !removing}

	user, err := app.Queries.UpdateUserPin(r.Context(), database.UpdateUserPinParams{
		Pin: pinValue,
		ID:  userID,
	})
	if err != nil {
		app.Logger.Error("failed to update user pin", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	message := "PIN updated successfully"
	if removing {
		message = "PIN removed successfully"
	}

	app.Logger.Info("user pin updated", "user_id", userID, "removed", removing)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: message,
		Data: map[string]any{
			"user": userResponseMap(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin, user.CreatedAt, user.UpdatedAt),
		},
	})
}

// GetUserPin returns the plaintext PIN to its owner. Session cookie only —
// a device bearer token from a shared TV must never be able to read it.
func (app *Application) GetUserPin(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user for pin read", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		}
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"pin": helpers.StringPtrFromNull(user.Pin),
		},
	})
}

type VerifyUserPinRequest struct {
	Pin string `json:"pin"`
}

// VerifyUserPin checks a PIN for the TV profile-switch flow. A wrong PIN is
// an expected domain outcome, so both match and mismatch return 200 with
// data.valid — 401 stays reserved for missing/revoked credentials.
func (app *Application) VerifyUserPin(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return
	}

	var req VerifyUserPinRequest
	if err := helpers.ReadJSON(w, r, &req, 0); err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	if err := validatePin(req.Pin); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		} else {
			app.Logger.Error("failed to fetch user for pin verification", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		}
		return
	}

	if !user.Pin.Valid {
		helpers.ErrorJSON(w, errors.New("no PIN is set for this account"), http.StatusBadRequest)
		return
	}

	if !app.allowPinAttempt(userID) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	valid := req.Pin == user.Pin.String

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"valid": valid,
		},
	})
}
