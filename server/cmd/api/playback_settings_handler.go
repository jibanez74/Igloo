package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

const subtitleLanguageOff = "off"

var languageCodePattern = regexp.MustCompile(`^[a-z]{2,3}$`)

type playbackProfileResponse struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Height    int    `json:"height"`
	VideoMbps int    `json:"video_mbps"`
}

type playbackSettingsResponse struct {
	Profiles                  []playbackProfileResponse `json:"profiles"`
	PreferredProfile          *string                   `json:"preferred_profile"`
	DownloadMbps              *float64                  `json:"download_mbps"`
	ServerUploadMbps          *float64                  `json:"server_upload_mbps"`
	IsAdmin                   bool                      `json:"is_admin"`
	PreferredAudioLanguage    *string                   `json:"preferred_audio_language"`
	PreferredSubtitleLanguage *string                   `json:"preferred_subtitle_language"`
}

// PUT returns only user-editable playback preferences; the client refetches the full catalog.
type updatePlaybackSettingsResponse struct {
	PreferredProfile          *string  `json:"preferred_profile"`
	DownloadMbps              *float64 `json:"download_mbps"`
	PreferredAudioLanguage    *string  `json:"preferred_audio_language"`
	PreferredSubtitleLanguage *string  `json:"preferred_subtitle_language"`
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

func (app *Application) GetPlaybackSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
			return
		}
		app.Logger.Error("failed to load playback preferences", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	var serverUpload *float64
	if app.Settings != nil {
		serverUpload = helpers.Float64PtrFromNull(app.Settings.ServerUploadMbps)
	}

	res := playbackSettingsResponse{
		Profiles:                  playbackProfileCatalog(),
		PreferredProfile:          helpers.StringPtrFromNull(prefs.PreferredHlsProfile),
		DownloadMbps:              helpers.Float64PtrFromNull(prefs.DownloadMbps),
		ServerUploadMbps:          serverUpload,
		IsAdmin:                   prefs.IsAdmin,
		PreferredAudioLanguage:    helpers.StringPtrFromNull(prefs.PreferredAudioLanguage),
		PreferredSubtitleLanguage: helpers.StringPtrFromNull(prefs.PreferredSubtitleLanguage),
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"settings": res,
		},
	})
}

func (app *Application) UpdatePlaybackSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return
	}

	var rawFields map[string]json.RawMessage
	err := helpers.ReadJSON(w, r, &rawFields, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	current, err := app.Queries.GetUserPlaybackPreferences(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to load playback preferences", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	if _, ok := rawFields["server_upload_mbps"]; ok && !current.IsAdmin {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusForbidden)
		return
	}

	preferred := current.PreferredHlsProfile
	if raw, ok := rawFields["preferred_profile"]; ok {
		var val *string
		if err := json.Unmarshal(raw, &val); err != nil {
			helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
			return
		}
		preferred = sql.NullString{}
		if val != nil {
			trimmed := strings.TrimSpace(*val)
			if trimmed != "" {
				if !helpers.IsAllowedHLSProfile(trimmed) || trimmed == helpers.HLS_PROFILE_REMUX {
					helpers.ErrorJSON(w, errors.New("invalid playback profile"), http.StatusBadRequest)
					return
				}
				preferred = helpers.NullString(trimmed)
			}
		}
	}

	download := current.DownloadMbps
	if raw, ok := rawFields["download_mbps"]; ok {
		var val *float64
		if err := json.Unmarshal(raw, &val); err != nil {
			helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
			return
		}
		download = sql.NullFloat64{}
		if val != nil {
			if *val <= 0 || *val >= 10000 {
				helpers.ErrorJSON(w, errors.New("download speed must be greater than 0 and less than 10000 Mbps"), http.StatusBadRequest)
				return
			}
			download = helpers.NullFloat64FromPtr(val)
		}
	}

	audioLang := current.PreferredAudioLanguage
	if raw, ok := rawFields["preferred_audio_language"]; ok {
		var val *string
		if err := json.Unmarshal(raw, &val); err != nil {
			helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
			return
		}
		audioLang = sql.NullString{}
		if val != nil {
			trimmed := strings.TrimSpace(*val)
			if trimmed != "" {
				if trimmed == subtitleLanguageOff || !languageCodePattern.MatchString(trimmed) {
					helpers.ErrorJSON(w, errors.New("invalid audio language code"), http.StatusBadRequest)
					return
				}
				audioLang = helpers.NullString(trimmed)
			}
		}
	}

	subtitleLang := current.PreferredSubtitleLanguage
	if raw, ok := rawFields["preferred_subtitle_language"]; ok {
		var val *string
		if err := json.Unmarshal(raw, &val); err != nil {
			helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
			return
		}
		subtitleLang = sql.NullString{}
		if val != nil {
			trimmed := strings.TrimSpace(*val)
			if trimmed != "" {
				if trimmed != subtitleLanguageOff && !languageCodePattern.MatchString(trimmed) {
					helpers.ErrorJSON(w, errors.New("invalid subtitle language code"), http.StatusBadRequest)
					return
				}
				subtitleLang = helpers.NullString(trimmed)
			}
		}
	}

	var serverUpload sql.NullFloat64
	serverUploadProvided := false
	if raw, ok := rawFields["server_upload_mbps"]; ok {
		serverUploadProvided = true

		var val *float64
		if err := json.Unmarshal(raw, &val); err != nil {
			helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
			return
		}
		if val != nil {
			if *val <= 0 || *val >= 100000 {
				helpers.ErrorJSON(w, errors.New("server upload speed must be greater than 0 and less than 100000 Mbps"), http.StatusBadRequest)
				return
			}
			serverUpload = helpers.NullFloat64FromPtr(val)
		}
	}

	updateParams := database.UpdateUserPlaybackPreferencesParams{
		PreferredHlsProfile:       preferred,
		DownloadMbps:              download,
		PreferredAudioLanguage:    audioLang,
		PreferredSubtitleLanguage: subtitleLang,
		ID:                        userID,
	}

	var updated database.UpdateUserPlaybackPreferencesRow
	if serverUploadProvided {
		tx, err := app.DB.BeginTx(r.Context(), nil)
		if err != nil {
			app.Logger.Error("failed to begin playback settings transaction", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New("failed to update playback settings"))
			return
		}

		qtx := app.Queries.WithTx(tx)
		updated, err = qtx.UpdateUserPlaybackPreferences(r.Context(), updateParams)
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				app.Logger.Error("failed to roll back playback settings transaction", "error", rollbackErr, "user_id", userID)
			}
			app.Logger.Error("failed to update playback preferences", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New("failed to update playback settings"))
			return
		}

		updatedSettings, err := qtx.UpdatePlaybackServerUploadMbps(r.Context(), serverUpload)
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				app.Logger.Error("failed to roll back playback settings transaction", "error", rollbackErr, "user_id", userID)
			}
			app.Logger.Error("failed to update playback server upload speed", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New("failed to update playback settings"))
			return
		}

		err = tx.Commit()
		if err != nil {
			app.Logger.Error("failed to commit playback settings transaction", "error", err, "user_id", userID)
			helpers.ErrorJSON(w, errors.New("failed to update playback settings"))
			return
		}

		app.Settings = &updatedSettings
	} else {
		updated, err = app.Queries.UpdateUserPlaybackPreferences(r.Context(), updateParams)
	}
	if err != nil {
		app.Logger.Error("failed to update playback preferences", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to update playback settings"))
		return
	}

	res := updatePlaybackSettingsResponse{
		PreferredProfile:          helpers.StringPtrFromNull(updated.PreferredHlsProfile),
		DownloadMbps:              helpers.Float64PtrFromNull(updated.DownloadMbps),
		PreferredAudioLanguage:    helpers.StringPtrFromNull(updated.PreferredAudioLanguage),
		PreferredSubtitleLanguage: helpers.StringPtrFromNull(updated.PreferredSubtitleLanguage),
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Playback settings updated",
		Data: map[string]any{
			"settings": res,
		},
	})
}
