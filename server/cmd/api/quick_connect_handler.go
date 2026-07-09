package main

import (
	"database/sql"
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
	tooManyAttemptsMessage         = "too many attempts, please try again later"
	quickConnectBusyMessage        = "quick connect is busy, please try again later"

	maxDeviceNameLength = 100
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

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	request.DeviceName = strings.TrimSpace(request.DeviceName)
	if request.DeviceName == "" || len(request.DeviceName) > maxDeviceNameLength {
		helpers.ErrorJSON(w, errors.New("device_name is required and must be at most 100 characters"), http.StatusBadRequest)
		return
	}

	code, secret, err := app.QuickConnect.Initiate(request.DeviceName, request.Platform, request.AppVersion)
	if err != nil {
		if errors.Is(err, errQuickConnectCapacityReached) {
			helpers.ErrorJSON(w, errors.New(quickConnectBusyMessage), http.StatusServiceUnavailable)
			return
		}

		app.Logger.Error("failed to initiate quick connect", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	helpers.WriteJSON(w, http.StatusCreated, helpers.JSONResponse{
		Error: false,
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

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	code := normalizeQuickConnectCode(request.Code)
	result := app.QuickConnect.Redeem(code, request.Secret)
	switch result.status {
	case redeemPending:
		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data:  map[string]any{"status": "pending"},
		})
	case redeemApproved:
		token, device, ok := app.issueDeviceToken(w, r, result.userID, result.deviceName, result.platform, result.appVersion)
		if !ok {
			return
		}

		app.QuickConnect.Consume(code)

		helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
			Error: false,
			Data: map[string]any{
				"status": "approved",
				"token":  token,
				"device": deviceResponseMap(device.ID, device.Name, device.Platform, device.AppVersion, device.CreatedAt, device.LastUsedAt, true),
			},
		})
	default:
		helpers.ErrorJSON(w, errors.New(quickConnectInvalidCodeMessage), http.StatusNotFound)
	}
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

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	code := normalizeQuickConnectCode(request.Code)
	if !app.QuickConnect.Approve(code, userID) {
		helpers.ErrorJSON(w, errors.New(quickConnectInvalidCodeMessage), http.StatusNotFound)
		return
	}

	app.Logger.Info("quick connect code approved", "user_id", userID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Device approved. It will finish signing in shortly.",
	})
}

// issueDeviceToken creates the devices row for a freshly authenticated
// device. The plaintext token exists only in the caller's response; on
// failure an error response has already been written and ok is false.
func (app *Application) issueDeviceToken(w http.ResponseWriter, r *http.Request, userID int64, deviceName, platform, appVersion string) (string, database.Device, bool) {
	token, tokenHash, err := generateDeviceToken()
	if err != nil {
		app.Logger.Error("failed to generate device token", "error", err)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return "", database.Device{}, false
	}

	device, err := app.Queries.CreateDevice(r.Context(), database.CreateDeviceParams{
		UserID:     userID,
		Name:       deviceName,
		Platform:   platform,
		AppVersion: helpers.NullString(appVersion),
		TokenHash:  tokenHash,
	})
	if err != nil {
		app.Logger.Error("failed to create device", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return "", database.Device{}, false
	}

	app.Logger.Info("device token issued", "user_id", userID, "device_id", device.ID)

	return token, device, true
}

func normalizeQuickConnectCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func deviceResponseMap(id int64, name, platform string, appVersion sql.NullString, createdAt, lastUsedAt string, isCurrent bool) map[string]any {
	var version *string
	if appVersion.Valid {
		version = &appVersion.String
	}

	return map[string]any{
		"id":           id,
		"name":         name,
		"platform":     platform,
		"app_version":  version,
		"created_at":   createdAt,
		"last_used_at": lastUsedAt,
		"is_current":   isCurrent,
	}
}
