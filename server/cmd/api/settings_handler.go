package main

import (
	"database/sql"
	"errors"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	scanMutex  sync.Mutex
	isScanning bool

	movieScanMutex  sync.Mutex
	isMovieScanning bool
)

type generalSettingsResponse struct {
	TmdbKey                    *string  `json:"tmdb_key"`
	JellyfinToken              *string  `json:"jellyfin_token"`
	SpotifyClientID            *string  `json:"spotify_client_id"`
	SpotifyClientSecret        *string  `json:"spotify_client_secret"`
	HardwareAccelerationDevice string   `json:"hardware_acceleration_device"`
	EnableLogger               bool     `json:"enable_logger"`
	EnableWatcher              bool     `json:"enable_watcher"`
	DownloadImages             bool     `json:"download_images"`
	StaticDir                  string   `json:"static_dir"`
	LogsDir                    string   `json:"logs_dir"`
	TranscodeDir               string   `json:"transcode_dir"`
	ServerUploadMbps           *float64 `json:"server_upload_mbps"`
	RestartRequired            bool     `json:"restart_required,omitempty"`
}

type updateGeneralSettingsRequest struct {
	TmdbKey                    string   `json:"tmdb_key"`
	JellyfinToken              string   `json:"jellyfin_token"`
	SpotifyClientID            string   `json:"spotify_client_id"`
	SpotifyClientSecret        string   `json:"spotify_client_secret"`
	HardwareAccelerationDevice string   `json:"hardware_acceleration_device"`
	EnableLogger               bool     `json:"enable_logger"`
	EnableWatcher              bool     `json:"enable_watcher"`
	DownloadImages             bool     `json:"download_images"`
	StaticDir                  string   `json:"static_dir"`
	LogsDir                    string   `json:"logs_dir"`
	TranscodeDir               string   `json:"transcode_dir"`
	ServerUploadMbps           *float64 `json:"server_upload_mbps"`
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

func nullableStringValue(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	return &value.String
}

func nullableFloat64Value(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}

	return &value.Float64
}

func mapGeneralSettingsResponse(settings database.Setting, restartRequired bool) generalSettingsResponse {
	hardwareAccelerationDevice := helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	if settings.HardwareAccelerationDevice.Valid && settings.HardwareAccelerationDevice.String != "" {
		hardwareAccelerationDevice = settings.HardwareAccelerationDevice.String
	}

	return generalSettingsResponse{
		TmdbKey:                    nullableStringValue(settings.TmdbKey),
		JellyfinToken:              nullableStringValue(settings.JellyfinToken),
		SpotifyClientID:            nullableStringValue(settings.SpotifyClientID),
		SpotifyClientSecret:        nullableStringValue(settings.SpotifyClientSecret),
		HardwareAccelerationDevice: hardwareAccelerationDevice,
		EnableLogger:               settings.EnableLogger,
		EnableWatcher:              settings.EnableWatcher,
		DownloadImages:             settings.DownloadImages,
		StaticDir:                  settings.StaticDir,
		LogsDir:                    settings.LogsDir,
		TranscodeDir:               settings.TranscodeDir,
		ServerUploadMbps:           nullableFloat64Value(settings.ServerUploadMbps),
		RestartRequired:            restartRequired,
	}
}

func mapLibrarySettingsResponse(settings database.Setting) librarySettingsResponse {
	return librarySettingsResponse{
		MoviesDir: nullableStringValue(settings.MoviesDir),
		ShowsDir:  nullableStringValue(settings.ShowsDir),
		MusicDir:  nullableStringValue(settings.MusicDir),
	}
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
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
		return
	}

	req.TmdbKey = strings.TrimSpace(req.TmdbKey)
	req.JellyfinToken = strings.TrimSpace(req.JellyfinToken)
	req.SpotifyClientID = strings.TrimSpace(req.SpotifyClientID)
	req.SpotifyClientSecret = strings.TrimSpace(req.SpotifyClientSecret)
	req.HardwareAccelerationDevice = strings.TrimSpace(req.HardwareAccelerationDevice)
	req.StaticDir = strings.TrimSpace(req.StaticDir)
	req.LogsDir = strings.TrimSpace(req.LogsDir)
	req.TranscodeDir = strings.TrimSpace(req.TranscodeDir)

	if !validateHardwareAccelerationDevice(req.HardwareAccelerationDevice) {
		helpers.ErrorJSON(w, errors.New("invalid hardware acceleration device"), http.StatusBadRequest)
		return
	}

	if req.StaticDir == "" {
		helpers.ErrorJSON(w, errors.New("static directory is required"), http.StatusBadRequest)
		return
	}

	if req.LogsDir == "" {
		helpers.ErrorJSON(w, errors.New("logs directory is required"), http.StatusBadRequest)
		return
	}

	if req.TranscodeDir == "" {
		helpers.ErrorJSON(w, errors.New("transcode directory is required"), http.StatusBadRequest)
		return
	}

	if req.ServerUploadMbps != nil {
		if *req.ServerUploadMbps <= 0 || *req.ServerUploadMbps >= 100000 {
			helpers.ErrorJSON(w, errors.New("server upload speed must be greater than 0 and less than 100000 Mbps"), http.StatusBadRequest)
			return
		}
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

	_, err = helpers.GetOrCreateDir(req.LogsDir)
	if err != nil {
		app.Logger.Error("failed to validate logs directory", "error", err, "path", req.LogsDir)
		helpers.ErrorJSON(w, errors.New("logs directory is not accessible"), http.StatusBadRequest)
		return
	}

	_, err = helpers.GetOrCreateDir(req.TranscodeDir)
	if err != nil {
		app.Logger.Error("failed to validate transcode directory", "error", err, "path", req.TranscodeDir)
		helpers.ErrorJSON(w, errors.New("transcode directory is not accessible"), http.StatusBadRequest)
		return
	}

	serverUploadMbps := sql.NullFloat64{}
	if req.ServerUploadMbps != nil {
		serverUploadMbps = sql.NullFloat64{Float64: *req.ServerUploadMbps, Valid: true}
	}

	currentSettings := app.Settings
	updatedSettings, err := app.Queries.UpdateGeneralSettings(r.Context(), database.UpdateGeneralSettingsParams{
		TmdbKey:                    helpers.NullString(req.TmdbKey),
		JellyfinToken:              helpers.NullString(req.JellyfinToken),
		SpotifyClientID:            helpers.NullString(req.SpotifyClientID),
		SpotifyClientSecret:        helpers.NullString(req.SpotifyClientSecret),
		HardwareAccelerationDevice: helpers.NullString(req.HardwareAccelerationDevice),
		EnableLogger:               req.EnableLogger,
		EnableWatcher:              req.EnableWatcher,
		DownloadImages:             req.DownloadImages,
		StaticDir:                  req.StaticDir,
		LogsDir:                    req.LogsDir,
		TranscodeDir:               req.TranscodeDir,
		ServerUploadMbps:           serverUploadMbps,
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
		previous.LogsDir != next.LogsDir ||
		previous.TranscodeDir != next.TranscodeDir ||
		previous.EnableLogger != next.EnableLogger ||
		previous.TmdbKey != next.TmdbKey ||
		previous.JellyfinToken != next.JellyfinToken ||
		previous.SpotifyClientID != next.SpotifyClientID ||
		previous.SpotifyClientSecret != next.SpotifyClientSecret
}

func (app *Application) UpdateLibrarySettings(w http.ResponseWriter, r *http.Request) {
	var req updateLibrarySettingsRequest
	err := helpers.ReadJSON(w, r, &req, 0)
	if err != nil {
		helpers.ErrorJSON(w, errors.New("invalid request body"), http.StatusBadRequest)
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
	scanMutex.Lock()
	if isScanning {
		scanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("music library scan is already in progress"))
		return
	}

	isScanning = true
	scanMutex.Unlock()

	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String == "" {
		scanMutex.Lock()
		isScanning = false
		scanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("music directory is not configured"))
		return
	}

	go func() {
		defer func() {
			scanMutex.Lock()
			isScanning = false
			scanMutex.Unlock()
		}()

		app.ScanMusicLibrary()
	}()

	app.Logger.Info("music library scan triggered via API", "path", app.Settings.MusicDir.String)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Music library scan started",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}

func (app *Application) TriggerMovieScan(w http.ResponseWriter, r *http.Request) {
	movieScanMutex.Lock()
	if isMovieScanning {
		movieScanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("movie library scan is already in progress"))
		return
	}

	isMovieScanning = true
	movieScanMutex.Unlock()

	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		movieScanMutex.Lock()
		isMovieScanning = false
		movieScanMutex.Unlock()
		helpers.ErrorJSON(w, errors.New("movies directory is not configured"))
		return
	}

	go func() {
		defer func() {
			movieScanMutex.Lock()
			isMovieScanning = false
			movieScanMutex.Unlock()
		}()

		app.ScanMoviesLibrary()
	}()

	app.Logger.Info("movie library scan triggered via API", "path", app.Settings.MoviesDir.String)

	res := helpers.JSONResponse{
		Error:   false,
		Message: "Movie library scan started",
	}

	helpers.WriteJSON(w, http.StatusOK, res)
}
