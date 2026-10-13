package main

import (
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	quickConnectInvalidCodeMessage = "invalid or expired code"
	quickConnectPollSeconds        = 2
)

type InitiateQuickConnectRequest struct {
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

func (app *Application) InitiateQuickConnect(w http.ResponseWriter, r *http.Request) {
	if !app.AuthLimiter.Allow("initiate:"+clientIP(r), 10, time.Minute) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	var request InitiateQuickConnectRequest

	err := helpers.ReadJSON(w, r, &request)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	request.DeviceName = strings.TrimSpace(request.DeviceName)
	err = validateDeviceInfo(request.DeviceName, request.Platform, request.AppVersion)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	code, secret, err := app.QuickConnect.Initiate(request.DeviceName, request.Platform, request.AppVersion)
	if err != nil {
		if errors.Is(err, errQuickConnectCapacityReached) {
			helpers.ErrorJSON(w, errors.New("quick connect is busy, please try again later"), http.StatusServiceUnavailable)
			return
		}

		app.serverError(w, err, "failed to initiate quick connect", internalServerErrorMessage)
		return
	}

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Data: map[string]any{
			"code":                  code,
			"secret":                secret,
			"expires_in_seconds":    int(quickConnectCodeTTL.Seconds()),
			"poll_interval_seconds": quickConnectPollSeconds,
		},
	})
}

type RedeemQuickConnectRequest struct {
	Code   string `json:"code"`
	Secret string `json:"secret"`
}

func (app *Application) RedeemQuickConnect(w http.ResponseWriter, r *http.Request) {
	if !app.AuthLimiter.Allow("redeem:"+clientIP(r), 60, time.Minute) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	var request RedeemQuickConnectRequest

	err := helpers.ReadJSON(w, r, &request)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	code := normalizeQuickConnectCode(request.Code)
	result := app.QuickConnect.Redeem(code, request.Secret)
	switch result.status {
	case redeemPending:
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: map[string]any{"status": "pending"}})
	case redeemApproved:
		token, device, ok := app.issueDeviceToken(w, r, result.userID, result.deviceName, result.platform, result.appVersion)
		if !ok {
			return
		}

		app.QuickConnect.Consume(code)

		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Data: map[string]any{
				"status": "approved",
				"token":  token,
				"device": deviceResponseMap(database.GetDevicesByUserRow(device)),
			},
		})
	default:
		helpers.ErrorJSON(w, errors.New(quickConnectInvalidCodeMessage), http.StatusNotFound)
	}
}

type LookupQuickConnectRequest struct {
	Code string `json:"code"`
}

func (app *Application) LookupQuickConnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	// Shares the approve bucket so lookup cannot be used to guess codes
	// faster than approve alone would allow.
	if !app.AuthLimiter.Allow("approve:"+strconv.FormatInt(userID, 10), 10, 5*time.Minute) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	var request LookupQuickConnectRequest

	err := helpers.ReadJSON(w, r, &request)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	code := normalizeQuickConnectCode(request.Code)
	info, found := app.QuickConnect.Lookup(code)
	if !found {
		helpers.ErrorJSON(w, errors.New(quickConnectInvalidCodeMessage), http.StatusNotFound)
		return
	}

	var version *string
	if info.appVersion != "" {
		version = &info.appVersion
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{
			"device_name": info.deviceName,
			"platform":    info.platform,
			"app_version": version,
		},
	})
}

type ApproveQuickConnectRequest struct {
	Code string `json:"code"`
}

func (app *Application) ApproveQuickConnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	if !app.AuthLimiter.Allow("approve:"+strconv.FormatInt(userID, 10), 10, 5*time.Minute) {
		helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
		return
	}

	var request ApproveQuickConnectRequest

	err := helpers.ReadJSON(w, r, &request)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	code := normalizeQuickConnectCode(request.Code)
	if !app.QuickConnect.Approve(code, userID) {
		helpers.ErrorJSON(w, errors.New(quickConnectInvalidCodeMessage), http.StatusNotFound)
		return
	}

	app.Logger.Info("quick connect code approved", "user_id", userID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Device approved. It will finish signing in shortly.",
	})
}

// issueDeviceToken creates the devices row for a freshly authenticated
// device. The plaintext token exists only in the caller's response; on
// failure an error response has already been written and ok is false.
func (app *Application) issueDeviceToken(w http.ResponseWriter, r *http.Request, userID int64, deviceName, platform, appVersion string) (string, database.CreateDeviceRow, bool) {
	token, tokenHash := generateDeviceToken()
	device, err := app.Queries.CreateDevice(r.Context(), database.CreateDeviceParams{
		UserID:     userID,
		Name:       deviceName,
		Platform:   platform,
		AppVersion: helpers.NullString(appVersion),
		TokenHash:  tokenHash,
	})
	if err != nil {
		app.serverError(w, err, "failed to create device", internalServerErrorMessage, "user_id", userID)
		return "", database.CreateDeviceRow{}, false
	}

	app.Logger.Info("device token issued", "user_id", userID, "device_id", device.ID)

	return token, device, true
}

// normalizeQuickConnectCode accepts a code the way a person reads it off a
// screen: any casing, and with the spacing or dashes a client may use to make
// it legible. Only separators are dropped. Removing every character outside the
// alphabet would let a mistyped "ABC2340" collapse into a valid code.
func normalizeQuickConnectCode(code string) string {
	stripSeparators := func(r rune) rune {
		isSeparator := r == ' ' || r == '\t' || r == '-'
		if isSeparator {
			return -1
		}

		return r
	}

	return strings.Map(stripSeparators, strings.ToUpper(strings.TrimSpace(code)))
}

func deviceResponseMap(device database.GetDevicesByUserRow) map[string]any {
	return map[string]any{
		"id":           device.ID,
		"name":         device.Name,
		"platform":     device.Platform,
		"app_version":  helpers.StringPtrFromNull(device.AppVersion),
		"created_at":   device.CreatedAt,
		"last_used_at": device.LastUsedAt,
	}
}
