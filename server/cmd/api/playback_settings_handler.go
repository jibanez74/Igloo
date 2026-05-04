package main

import (
	"database/sql"
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

type updatePlaybackSettingsRequest struct {
	PreferredProfile          *string  `json:"preferred_profile"`
	DownloadMbps              *float64 `json:"download_mbps"`
	PreferredAudioLanguage    *string  `json:"preferred_audio_language"`
	PreferredSubtitleLanguage *string  `json:"preferred_subtitle_language"`
}

// updatePlaybackSettingsResponse is the PUT-only payload — the catalog,
// server upload cap, and is_admin don't change as a result of an update,
// so the client refetches the full GET payload via cache invalidation.
type updatePlaybackSettingsResponse struct {
	PreferredProfile          *string  `json:"preferred_profile"`
	DownloadMbps              *float64 `json:"download_mbps"`
	PreferredAudioLanguage    *string  `json:"preferred_audio_language"`
	PreferredSubtitleLanguage *string  `json:"preferred_subtitle_language"`
}

// playbackProfileCatalog returns the transcode profiles exposed to clients.
// HLS_PROFILE_REMUX is excluded because it has no resolution/bitrate constraints
// and is not meaningful for a "preferred profile" selection.
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
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	user, err := app.Queries.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
			return
		}
		app.Logger.Error("failed to load user for playback settings", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	prefs, err := app.Queries.GetUserPlaybackPreferences(r.Context(), userID)
	if err != nil {
		app.Logger.Error("failed to load playback preferences", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(helpers.INTERNAL_SERVER_ERROR))
		return
	}

	var serverUpload *float64
	if app.Settings != nil && app.Settings.ServerUploadMbps.Valid {
		v := app.Settings.ServerUploadMbps.Float64
		serverUpload = &v
	}

	res := playbackSettingsResponse{
		Profiles:                  playbackProfileCatalog(),
		PreferredProfile:          nullableStringValue(prefs.PreferredHlsProfile),
		DownloadMbps:              nullableFloat64Value(prefs.DownloadMbps),
		ServerUploadMbps:          serverUpload,
		IsAdmin:                   user.IsAdmin,
		PreferredAudioLanguage:    nullableStringValue(prefs.PreferredAudioLanguage),
		PreferredSubtitleLanguage: nullableStringValue(prefs.PreferredSubtitleLanguage),
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"settings": res,
		},
	})
}

func (app *Application) UpdatePlaybackSettings(w http.ResponseWriter, r *http.Request) {
	userID := app.SessionManager.GetInt64(r.Context(), helpers.COOKIE_USER_ID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(helpers.NOT_AUTHORIZED_MESSAGE), http.StatusUnauthorized)
		return
	}

	var req updatePlaybackSettingsRequest
	err := helpers.ReadJSON(w, r, &req, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	preferred := sql.NullString{}
	if req.PreferredProfile != nil {
		trimmed := strings.TrimSpace(*req.PreferredProfile)
		if trimmed != "" {
			if !helpers.IsAllowedHLSProfile(trimmed) || trimmed == helpers.HLS_PROFILE_REMUX {
				helpers.ErrorJSON(w, errors.New("invalid playback profile"), http.StatusBadRequest)
				return
			}
			preferred = sql.NullString{String: trimmed, Valid: true}
		}
	}

	download := sql.NullFloat64{}
	if req.DownloadMbps != nil {
		if *req.DownloadMbps <= 0 || *req.DownloadMbps >= 10000 {
			helpers.ErrorJSON(w, errors.New("download speed must be greater than 0 and less than 10000 Mbps"), http.StatusBadRequest)
			return
		}
		download = sql.NullFloat64{Float64: *req.DownloadMbps, Valid: true}
	}

	audioLang := sql.NullString{}
	if req.PreferredAudioLanguage != nil {
		trimmed := strings.TrimSpace(*req.PreferredAudioLanguage)
		if trimmed != "" {
			if trimmed == subtitleLanguageOff || !languageCodePattern.MatchString(trimmed) {
				helpers.ErrorJSON(w, errors.New("invalid audio language code"), http.StatusBadRequest)
				return
			}
			audioLang = sql.NullString{String: trimmed, Valid: true}
		}
	}

	subtitleLang := sql.NullString{}
	if req.PreferredSubtitleLanguage != nil {
		trimmed := strings.TrimSpace(*req.PreferredSubtitleLanguage)
		if trimmed != "" {
			if trimmed != subtitleLanguageOff && !languageCodePattern.MatchString(trimmed) {
				helpers.ErrorJSON(w, errors.New("invalid subtitle language code"), http.StatusBadRequest)
				return
			}
			subtitleLang = sql.NullString{String: trimmed, Valid: true}
		}
	}

	updated, err := app.Queries.UpdateUserPlaybackPreferences(r.Context(), database.UpdateUserPlaybackPreferencesParams{
		PreferredHlsProfile:       preferred,
		DownloadMbps:              download,
		PreferredAudioLanguage:    audioLang,
		PreferredSubtitleLanguage: subtitleLang,
		ID:                        userID,
	})
	if err != nil {
		app.Logger.Error("failed to update playback preferences", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New("failed to update playback settings"))
		return
	}

	res := updatePlaybackSettingsResponse{
		PreferredProfile:          nullableStringValue(updated.PreferredHlsProfile),
		DownloadMbps:              nullableFloat64Value(updated.DownloadMbps),
		PreferredAudioLanguage:    nullableStringValue(updated.PreferredAudioLanguage),
		PreferredSubtitleLanguage: nullableStringValue(updated.PreferredSubtitleLanguage),
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Playback settings updated",
		Data: map[string]any{
			"settings": res,
		},
	})
}
