package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type generalSettingsResponse struct {
	TmdbKey             *string `json:"tmdb_key"`
	ImmichBaseURL       *string `json:"immich_base_url"`
	ImmichApiKey        *string `json:"immich_api_key"`
	JellyfinBaseURL     *string `json:"jellyfin_base_url"`
	JellyfinApiKey      *string `json:"jellyfin_api_key"`
	SpotifyClientID     *string `json:"spotify_client_id"`
	SpotifyClientSecret *string `json:"spotify_client_secret"`
	EnableWatcher       bool    `json:"enable_watcher"`
	DownloadImages      bool    `json:"download_images"`
	StaticDir           string  `json:"static_dir"`
	TranscodeDir        string  `json:"transcode_dir"`
	RestartRequired     bool    `json:"restart_required,omitempty"`
}

type updateGeneralSettingsRequest struct {
	TmdbKey             string `json:"tmdb_key"`
	ImmichBaseURL       string `json:"immich_base_url"`
	ImmichApiKey        string `json:"immich_api_key"`
	JellyfinBaseURL     string `json:"jellyfin_base_url"`
	JellyfinApiKey      string `json:"jellyfin_api_key"`
	SpotifyClientID     string `json:"spotify_client_id"`
	SpotifyClientSecret string `json:"spotify_client_secret"`
	EnableWatcher       bool   `json:"enable_watcher"`
	DownloadImages      bool   `json:"download_images"`
	StaticDir           string `json:"static_dir"`
	TranscodeDir        string `json:"transcode_dir"`
}

type librarySettingsResponse struct {
	MoviesDir *string `json:"movies_dir"`
	ShowsDir  *string `json:"shows_dir"`
	MusicDir  *string `json:"music_dir"`
}

type updateLibrarySettingsRequest struct {
	MoviesDir *string `json:"movies_dir"`
	ShowsDir  *string `json:"shows_dir"`
	MusicDir  *string `json:"music_dir"`
}

func mapGeneralSettingsResponse(settings database.Setting, restartRequired bool) generalSettingsResponse {
	return generalSettingsResponse{
		TmdbKey:             helpers.StringPtrFromNull(settings.TmdbKey),
		ImmichBaseURL:       helpers.StringPtrFromNull(settings.ImmichBaseUrl),
		ImmichApiKey:        helpers.StringPtrFromNull(settings.ImmichApiKey),
		JellyfinBaseURL:     helpers.StringPtrFromNull(settings.JellyfinBaseUrl),
		JellyfinApiKey:      helpers.StringPtrFromNull(settings.JellyfinApiKey),
		SpotifyClientID:     helpers.StringPtrFromNull(settings.SpotifyClientID),
		SpotifyClientSecret: helpers.StringPtrFromNull(settings.SpotifyClientSecret),
		EnableWatcher:       settings.EnableWatcher,
		DownloadImages:      settings.DownloadImages,
		StaticDir:           settings.StaticDir,
		TranscodeDir:        settings.TranscodeDir,
		RestartRequired:     restartRequired,
	}
}

func mapLibrarySettingsResponse(settings database.Setting) librarySettingsResponse {
	return librarySettingsResponse{
		MoviesDir: helpers.StringPtrFromNull(settings.MoviesDir),
		ShowsDir:  helpers.StringPtrFromNull(settings.ShowsDir),
		MusicDir:  helpers.StringPtrFromNull(settings.MusicDir),
	}
}

func isOptionalHTTPBaseURL(value string) bool {
	if value == "" {
		return true
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	return parsed.Host != ""
}

func (app *Application) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	settings, err := app.Queries.GetSettings(ctx)
	if err != nil {
		app.Logger.Error("failed to get settings", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch settings"))
		return
	}

	res := helpers.JSONResponse{
		Error: false,
		Data:  mapLibrarySettingsResponse(settings),
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) GetGeneralSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := app.Queries.GetSettings(r.Context())
	if err != nil {
		app.Logger.Error("failed to get general settings", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to fetch settings"))
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error: false,
		Data: map[string]any{
			"settings": mapGeneralSettingsResponse(settings, false),
		},
	})
}

func (app *Application) UpdateGeneralSettings(w http.ResponseWriter, r *http.Request) {
	var req updateGeneralSettingsRequest
	err := helpers.ReadJSON(w, r, &req, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	req.TmdbKey = strings.TrimSpace(req.TmdbKey)
	req.ImmichBaseURL = strings.TrimSpace(req.ImmichBaseURL)
	req.ImmichApiKey = strings.TrimSpace(req.ImmichApiKey)
	req.JellyfinBaseURL = strings.TrimSpace(req.JellyfinBaseURL)
	req.JellyfinApiKey = strings.TrimSpace(req.JellyfinApiKey)
	req.SpotifyClientID = strings.TrimSpace(req.SpotifyClientID)
	req.SpotifyClientSecret = strings.TrimSpace(req.SpotifyClientSecret)
	req.StaticDir = strings.TrimSpace(req.StaticDir)
	req.TranscodeDir = strings.TrimSpace(req.TranscodeDir)

	if !isOptionalHTTPBaseURL(req.JellyfinBaseURL) {
		helpers.ErrorJSON(w, errors.New("jellyfin base URL must be a valid http or https URL"), http.StatusBadRequest)
		return
	}

	if !isOptionalHTTPBaseURL(req.ImmichBaseURL) {
		helpers.ErrorJSON(w, errors.New("immich base URL must be a valid http or https URL"), http.StatusBadRequest)
		return
	}

	if req.StaticDir == "" {
		helpers.ErrorJSON(w, errors.New("static directory is required"), http.StatusBadRequest)
		return
	}

	if req.TranscodeDir == "" {
		helpers.ErrorJSON(w, errors.New("transcode directory is required"), http.StatusBadRequest)
		return
	}

	_, err = helpers.GetOrCreateDir(req.StaticDir)
	if err != nil {
		app.Logger.Error("failed to validate static directory", "error", err, "path", req.StaticDir)
		helpers.ErrorJSON(w, errors.New("static directory is not accessible"), http.StatusBadRequest)
		return
	}

	_, err = helpers.GetOrCreateDir(filepath.Join(req.StaticDir, "albums"))
	if err != nil {
		app.Logger.Error("failed to validate static albums directory", "error", err, "path", req.StaticDir)
		helpers.ErrorJSON(w, errors.New("static directory is not accessible"), http.StatusBadRequest)
		return
	}

	_, err = helpers.GetOrCreateDir(filepath.Join(req.StaticDir, "musicians"))
	if err != nil {
		app.Logger.Error("failed to validate static musicians directory", "error", err, "path", req.StaticDir)
		helpers.ErrorJSON(w, errors.New("static directory is not accessible"), http.StatusBadRequest)
		return
	}

	_, err = helpers.GetOrCreateDir(req.TranscodeDir)
	if err != nil {
		app.Logger.Error("failed to validate transcode directory", "error", err, "path", req.TranscodeDir)
		helpers.ErrorJSON(w, errors.New("transcode directory is not accessible"), http.StatusBadRequest)
		return
	}

	currentSettings := app.Settings
	updatedSettings, err := app.Queries.UpdateGeneralSettings(r.Context(), database.UpdateGeneralSettingsParams{
		TmdbKey:             helpers.NullString(req.TmdbKey),
		ImmichBaseUrl:       helpers.NullString(req.ImmichBaseURL),
		ImmichApiKey:        helpers.NullString(req.ImmichApiKey),
		JellyfinBaseUrl:     helpers.NullString(req.JellyfinBaseURL),
		JellyfinApiKey:      helpers.NullString(req.JellyfinApiKey),
		SpotifyClientID:     helpers.NullString(req.SpotifyClientID),
		SpotifyClientSecret: helpers.NullString(req.SpotifyClientSecret),
		EnableWatcher:       req.EnableWatcher,
		DownloadImages:      req.DownloadImages,
		StaticDir:           req.StaticDir,
		TranscodeDir:        req.TranscodeDir,
	})
	if err != nil {
		app.Logger.Error("failed to update general settings", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update settings"))
		return
	}

	app.Settings = &updatedSettings
	restartRequired := generalSettingsRestartRequired(currentSettings, updatedSettings)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Settings updated",
		Data: map[string]any{
			"settings":         mapGeneralSettingsResponse(updatedSettings, restartRequired),
			"restart_required": restartRequired,
		},
	})
}

func generalSettingsRestartRequired(previous *database.Setting, next database.Setting) bool {
	if previous == nil {
		return false
	}

	return previous.StaticDir != next.StaticDir ||
		previous.TranscodeDir != next.TranscodeDir ||
		previous.TmdbKey != next.TmdbKey ||
		previous.ImmichBaseUrl != next.ImmichBaseUrl ||
		previous.ImmichApiKey != next.ImmichApiKey ||
		previous.JellyfinBaseUrl != next.JellyfinBaseUrl ||
		previous.JellyfinApiKey != next.JellyfinApiKey ||
		previous.SpotifyClientID != next.SpotifyClientID ||
		previous.SpotifyClientSecret != next.SpotifyClientSecret
}

func (app *Application) UpdateLibrarySettings(w http.ResponseWriter, r *http.Request) {
	var req updateLibrarySettingsRequest
	err := helpers.ReadJSON(w, r, &req, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	moviesDir, err := validatedOptionalMediaDir(req.MoviesDir)
	if err != nil {
		app.Logger.Error("failed to validate movies directory", "error", err)
		helpers.ErrorJSON(w, errors.New("movies directory is not accessible"), http.StatusBadRequest)
		return
	}

	showsDir, err := validatedOptionalMediaDir(req.ShowsDir)
	if err != nil {
		app.Logger.Error("failed to validate shows directory", "error", err)
		helpers.ErrorJSON(w, errors.New("shows directory is not accessible"), http.StatusBadRequest)
		return
	}

	musicDir, err := validatedOptionalMediaDir(req.MusicDir)
	if err != nil {
		app.Logger.Error("failed to validate music directory", "error", err)
		helpers.ErrorJSON(w, errors.New("music directory is not accessible"), http.StatusBadRequest)
		return
	}

	updatedSettings, err := app.Queries.UpdateLibrarySettings(r.Context(), database.UpdateLibrarySettingsParams{
		MoviesDir: moviesDir,
		ShowsDir:  showsDir,
		MusicDir:  musicDir,
	})
	if err != nil {
		app.Logger.Error("failed to update library settings", "error", err)
		helpers.ErrorJSON(w, errors.New("failed to update library settings"))
		return
	}

	app.Settings = &updatedSettings

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Error:   false,
		Message: "Library settings updated",
		Data: map[string]any{
			"settings": mapLibrarySettingsResponse(updatedSettings),
		},
	})
}

func validatedOptionalMediaDir(value *string) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}

	dir := strings.TrimSpace(*value)
	if dir == "" {
		return sql.NullString{}, nil
	}

	info, err := os.Stat(dir)
	if err != nil {
		return sql.NullString{}, err
	}
	if !info.IsDir() {
		return sql.NullString{}, errors.New("path is not a directory")
	}

	return helpers.NullString(dir), nil
}

func (app *Application) TriggerMusicScan(w http.ResponseWriter, r *http.Request) {
	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		helpers.ErrorJSON(w, errors.New("music directory is not configured"))
		return
	}

	if !musicScanGuard.TryBegin() {
		helpers.ErrorJSON(w, errors.New("music library scan is already in progress"), http.StatusConflict)
		return
	}

	if app.Wait != nil {
		app.Wait.Add(1)
	}
	go app.runMusicScan()

	app.Logger.Info("music library scan triggered via API", "path", app.Settings.MusicDir.String)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Music library scan started",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) TriggerMovieScan(w http.ResponseWriter, r *http.Request) {
	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		helpers.ErrorJSON(w, errors.New("movies directory is not configured"))
		return
	}

	if !movieScanGuard.TryBegin() {
		helpers.ErrorJSON(w, errors.New("movie library scan is already in progress"), http.StatusConflict)
		return
	}

	if app.Wait != nil {
		app.Wait.Add(1)
	}
	go app.runMovieScan()

	app.Logger.Info("movie library scan triggered via API", "path", app.Settings.MoviesDir.String)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Movie library scan started",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
