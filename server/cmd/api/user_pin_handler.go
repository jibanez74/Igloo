package main

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

const (
	userPinLength    = 4
	pinAttemptLimit  = 5
	pinAttemptWindow = time.Minute
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
	// A pointer so a missing pin is a 400 rather than a silent removal.
	Pin        *string `json:"pin"`
	CurrentPin string  `json:"current_pin"`
}

// UpdateUserPin sets, changes, or removes the profile PIN. An empty pin
// removes it (mirroring the avatar convention). Device tokens are allowed so
// a TV client can set the PIN on first sign-in, but changing or removing an
// existing PIN requires the current PIN.
func (app *Application) UpdateUserPin(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	var req UpdateUserPinRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if req.Pin == nil {
		helpers.ErrorJSON(w, errors.New("pin is required"), http.StatusBadRequest)
		return
	}
	newPin := *req.Pin

	removing := newPin == ""
	if !removing {
		if err := validatePin(newPin); err != nil {
			helpers.ErrorJSON(w, err, http.StatusBadRequest)
			return
		}
	}

	if req.CurrentPin != "" {
		err := validatePin(req.CurrentPin)
		if err != nil {
			helpers.ErrorJSON(w, errors.New("current PIN must be exactly 4 digits"), http.StatusBadRequest)
			return
		}
	}

	currentPin, err := app.Queries.GetUserPin(r.Context(), userID)
	if err != nil {
		app.userLookupFailed(w, err, "failed to fetch user for pin update", userID)
		return
	}

	if currentPin.Valid {
		if !app.allowPinAttempt(userID) {
			helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
			return
		}

		if req.CurrentPin == "" {
			helpers.ErrorJSON(w, errors.New("current PIN is required"), http.StatusBadRequest)
			return
		}

		if req.CurrentPin != currentPin.String {
			helpers.ErrorJSON(w, errors.New("current PIN is incorrect"), http.StatusUnauthorized)
			return
		}
	}

	user, err := app.Queries.UpdateUserPin(r.Context(), database.UpdateUserPinParams{
		Pin: helpers.NullString(newPin),
		ID:  userID,
	})
	if err != nil {
		app.serverError(w, err, "failed to update user pin", internalServerErrorMessage, "user_id", userID)
		return
	}

	message := "PIN updated successfully"
	if removing {
		message = "PIN removed successfully"
	}

	app.Logger.Info("user pin updated", "user_id", userID, "removed", removing)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: message,
		Data: map[string]any{
			"user": newUserResponse(user.ID, user.Name, user.Email, user.IsAdmin, user.Avatar, user.Pin.Valid, user.CreatedAt, user.UpdatedAt),
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

	pin, err := app.Queries.GetUserPin(r.Context(), userID)
	if err != nil {
		app.userLookupFailed(w, err, "failed to fetch user for pin read", userID)
		return
	}

	w.Header().Set("Cache-Control", "private, no-store")
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{"pin": helpers.StringPtrFromNull(pin)},
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

	var req VerifyUserPinRequest
	if err := helpers.ReadJSON(w, r, &req); err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	if err := validatePin(req.Pin); err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	pin, err := app.Queries.GetUserPin(r.Context(), userID)
	if err != nil {
		app.userLookupFailed(w, err, "failed to fetch user for pin verification", userID)
		return
	}

	if !pin.Valid {
		helpers.ErrorJSON(w, errors.New("no PIN is set for this account"), http.StatusBadRequest)
		return
	}

	if !app.allowPinAttempt(userID) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	valid := req.Pin == pin.String

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"valid": valid}})
}
