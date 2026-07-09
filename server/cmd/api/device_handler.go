package main

import (
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (app *Application) GetDevices(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	rows, err := app.Queries.GetDevicesByUser(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to list devices", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	devices := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		// The list is session-only, so no device in it is ever the caller;
		// is_current stays in the shape for the redeem/device-login envelopes.
		devices = append(devices, deviceResponseMap(
			row.ID,
			row.Name,
			row.Platform,
			row.AppVersion,
			row.CreatedAt,
			row.LastUsedAt,
			false,
		))
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data:  map[string]any{"devices": devices},
	})
}

type RenameDeviceRequest struct {
	Name string `json:"name"`
}

func (app *Application) RenameDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	deviceID, ok := parseDeviceID(w, r)
	if !ok {
		return
	}

	var request RenameDeviceRequest

	err := helpers.ReadJSON(w, r, &request, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > maxDeviceNameLength {
		helpers.ErrorJSON(w, errors.New("name is required and must be at most 100 characters"), http.StatusBadRequest)
		return
	}

	rowsAffected, err := app.Queries.RenameDevice(r.Context(), database.RenameDeviceParams{
		Name:   request.Name,
		ID:     deviceID,
		UserID: userID,
	})
	if err != nil {
		app.Logger.Error("failed to rename device", "error", err, "user_id", userID, "device_id", deviceID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	if rowsAffected == 0 {
		helpers.ErrorJSON(w, errors.New("device not found"), http.StatusNotFound)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Device renamed",
	})
}

func (app *Application) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.requireSessionUserID(w, r)
	if !ok {
		return
	}

	deviceID, ok := parseDeviceID(w, r)
	if !ok {
		return
	}

	rowsAffected, err := app.Queries.DeleteDeviceForUser(r.Context(), database.DeleteDeviceForUserParams{
		ID:     deviceID,
		UserID: userID,
	})
	if err != nil {
		app.Logger.Error("failed to revoke device", "error", err, "user_id", userID, "device_id", deviceID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	if rowsAffected == 0 {
		helpers.ErrorJSON(w, errors.New("device not found"), http.StatusNotFound)
		return
	}

	app.DeviceLastSeen.Delete(strconv.FormatInt(deviceID, 10))

	app.Logger.Info("device revoked", "user_id", userID, "device_id", deviceID)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Device revoked",
	})
}

func parseDeviceID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		helpers.ErrorJSON(w, errors.New("invalid device id"), http.StatusBadRequest)
		return 0, false
	}

	return id, true
}
