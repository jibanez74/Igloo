package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

type playbackProfileResponse struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Height    int    `json:"height"`
	VideoMbps int    `json:"video_mbps"`
}

// Server-owned playback data. Per-device preferences (preferred profile,
// download speed, preferred audio/subtitle language) live in the browser's
// localStorage, because one account streams from devices with different
// screens, decoders and network links.
type playbackSettingsResponse struct {
	Profiles                   []playbackProfileResponse `json:"profiles"`
	ServerUploadMbps           *float64                  `json:"server_upload_mbps"`
	HardwareAccelerationDevice string                    `json:"hardware_acceleration_device"`
}

func hardwareAccelerationDeviceOrDefault(settings database.Setting) string {
	if settings.HardwareAccelerationDevice.Valid &&
		settings.HardwareAccelerationDevice.String != "" {
		return settings.HardwareAccelerationDevice.String
	}

	return helpers.HARDWARE_ACCELERATION_DEVICE_CPU
}

func validateHardwareAccelerationDevice(value string) bool {
	switch value {
	case helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
		return true
	default:
		return false
	}
}

// playbackProfileCatalog returns the transcode profiles exposed to clients.
// Remux is excluded because it has no resolution or bitrate constraints.
func playbackProfileCatalog() []playbackProfileResponse {
	out := make([]playbackProfileResponse, 0, len(helpers.HLSAllowedProfiles))

	for _, id := range helpers.HLSAllowedProfiles {
		if id == helpers.HLS_PROFILE_REMUX {
			continue
		}

		cfg, ok := helpers.HLSProfileConfigs[id]
		if !ok {
			continue
		}

		mbps, err := parseVideoMbps(cfg.VideoBitrate)
		if err != nil {
			continue
		}

		out = append(out, playbackProfileResponse{
			ID:        cfg.ID,
			Label:     formatProfileLabel(cfg.Height, mbps),
			Height:    cfg.Height,
			VideoMbps: mbps,
		})
	}

	return out
}

func parseVideoMbps(videoBitrate string) (int, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(videoBitrate), "M")
	if trimmed == "" {
		return 0, errors.New("empty video bitrate")
	}

	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}

	return n, nil
}

func formatProfileLabel(height, videoMbps int) string {
	return strconv.Itoa(height) + "p · " + strconv.Itoa(videoMbps) + " Mbps"
}

func mapPlaybackSettingsResponse(settings database.Setting) playbackSettingsResponse {
	return playbackSettingsResponse{
		Profiles:                   playbackProfileCatalog(),
		ServerUploadMbps:           helpers.Float64PtrFromNull(settings.ServerUploadMbps),
		HardwareAccelerationDevice: hardwareAccelerationDeviceOrDefault(settings),
	}
}

func (app *Application) GetPlaybackSettings(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"settings": mapPlaybackSettingsResponse(*app.CurrentSettings()),
		},
	})
}

// UpdatePlaybackSettings writes the server-wide playback settings. The route is
// admin-gated by RequireAdmin. Per-device preferences are not stored here --
// they live in the client's local storage. Fields absent from the body keep
// their current value, so a client may send just the one it changed.
func (app *Application) UpdatePlaybackSettings(w http.ResponseWriter, r *http.Request) {
	var rawFields map[string]json.RawMessage
	err := helpers.ReadJSON(w, r, &rawFields, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	// The body is validated before the lock is taken; only the fields that were
	// actually sent are decoded here, and the rest are filled in from the row
	// read under the lock below.
	var serverUpload *sql.NullFloat64
	if raw, ok := rawFields["server_upload_mbps"]; ok {
		var val *float64
		err := json.Unmarshal(raw, &val)
		if err != nil {
			helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
			return
		}

		parsed := sql.NullFloat64{}
		if val != nil {
			if *val <= 0 || *val >= 100000 {
				helpers.ErrorJSON(w, errors.New("server upload speed must be greater than 0 and less than 100000 Mbps"), http.StatusBadRequest)
				return
			}
			parsed = helpers.NullFloat64FromPtr(val)
		}
		serverUpload = &parsed
	}

	var hardwareDevice *string
	if raw, ok := rawFields["hardware_acceleration_device"]; ok {
		var val string
		err := json.Unmarshal(raw, &val)
		if err != nil {
			helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
			return
		}

		val = strings.TrimSpace(val)
		if !validateHardwareAccelerationDevice(val) {
			helpers.ErrorJSON(w, errors.New("invalid hardware acceleration device"), http.StatusBadRequest)
			return
		}
		hardwareDevice = &val
	}

	// The query writes both columns, so the omitted one has to be carried over
	// from the current row. Doing that under the write lock is what keeps two
	// concurrent partial updates from each restoring the other's stale value.
	updated, err := app.withSettingsWrite(r.Context(), func(current database.Setting) (database.Setting, error) {
		params := database.UpdatePlaybackServerSettingsParams{
			ServerUploadMbps:           current.ServerUploadMbps,
			HardwareAccelerationDevice: helpers.NullString(hardwareAccelerationDeviceOrDefault(current)),
		}
		if serverUpload != nil {
			params.ServerUploadMbps = *serverUpload
		}
		if hardwareDevice != nil {
			params.HardwareAccelerationDevice = helpers.NullString(*hardwareDevice)
		}

		return app.Queries.UpdatePlaybackServerSettings(r.Context(), params)
	})
	if err != nil {
		app.Logger.Error("failed to update playback server settings", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update playback settings"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Playback settings updated",
		Data: map[string]any{
			"settings": mapPlaybackSettingsResponse(updated),
		},
	})
}
