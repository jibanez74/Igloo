package main

import (
	"database/sql"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"net/http"
	"net/url"
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
}

// Every field is a pointer: the endpoint replaces the whole configuration, so
// a missing or null field is a 400 rather than a silent blank or false that
// would erase the stored value.
type updateGeneralSettingsRequest struct {
	TmdbKey             *string `json:"tmdb_key"`
	ImmichBaseURL       *string `json:"immich_base_url"`
	ImmichApiKey        *string `json:"immich_api_key"`
	JellyfinBaseURL     *string `json:"jellyfin_base_url"`
	JellyfinApiKey      *string `json:"jellyfin_api_key"`
	SpotifyClientID     *string `json:"spotify_client_id"`
	SpotifyClientSecret *string `json:"spotify_client_secret"`
	EnableWatcher       *bool   `json:"enable_watcher"`
	DownloadImages      *bool   `json:"download_images"`
	StaticDir           *string `json:"static_dir"`
	TranscodeDir        *string `json:"transcode_dir"`
}

// generalSettingsValues is the request with every field present and trimmed.
type generalSettingsValues struct {
	tmdbKey, immichBaseURL, immichApiKey, jellyfinBaseURL, jellyfinApiKey string
	spotifyClientID, spotifyClientSecret, staticDir, transcodeDir         string
	enableWatcher, downloadImages                                         bool
}

// missingField names the first required field the request
// leaves out or sets to null, or "" when every field is present.
func (req *updateGeneralSettingsRequest) missingField() string {
	stringFields := []struct {
		name  string
		value *string
	}{
		{"tmdb_key", req.TmdbKey},
		{"immich_base_url", req.ImmichBaseURL},
		{"immich_api_key", req.ImmichApiKey},
		{"jellyfin_base_url", req.JellyfinBaseURL},
		{"jellyfin_api_key", req.JellyfinApiKey},
		{"spotify_client_id", req.SpotifyClientID},
		{"spotify_client_secret", req.SpotifyClientSecret},
		{"static_dir", req.StaticDir},
		{"transcode_dir", req.TranscodeDir},
	}
	for _, field := range stringFields {
		if field.value == nil {
			return field.name
		}
	}
	if req.EnableWatcher == nil {
		return "enable_watcher"
	}
	if req.DownloadImages == nil {
		return "download_images"
	}

	return ""
}

func (req *updateGeneralSettingsRequest) values() generalSettingsValues {
	return generalSettingsValues{
		tmdbKey:             strings.TrimSpace(*req.TmdbKey),
		immichBaseURL:       strings.TrimSpace(*req.ImmichBaseURL),
		immichApiKey:        strings.TrimSpace(*req.ImmichApiKey),
		jellyfinBaseURL:     strings.TrimSpace(*req.JellyfinBaseURL),
		jellyfinApiKey:      strings.TrimSpace(*req.JellyfinApiKey),
		spotifyClientID:     strings.TrimSpace(*req.SpotifyClientID),
		spotifyClientSecret: strings.TrimSpace(*req.SpotifyClientSecret),
		staticDir:           strings.TrimSpace(*req.StaticDir),
		transcodeDir:        strings.TrimSpace(*req.TranscodeDir),
		enableWatcher:       *req.EnableWatcher,
		downloadImages:      *req.DownloadImages,
	}
}

// librarySettings is both the library settings response and the update request.
type librarySettings struct {
	MoviesDir *string `json:"movies_dir"`
	ShowsDir  *string `json:"shows_dir"`
	MusicDir  *string `json:"music_dir"`
}

func mapGeneralSettingsResponse(settings database.Setting) generalSettingsResponse {
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
	}
}

func mapLibrarySettingsResponse(settings database.Setting) librarySettings {
	return librarySettings{
		MoviesDir: helpers.StringPtrFromNull(settings.MoviesDir),
		ShowsDir:  helpers.StringPtrFromNull(settings.ShowsDir),
		MusicDir:  helpers.StringPtrFromNull(settings.MusicDir),
	}
}

func isOptionalHTTPURL(value string) bool {
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

// CurrentSettings returns the in-memory settings copy that every write path
// refreshes. The returned pointer is only ever replaced, never mutated in
// place, so callers may read it without holding the lock -- but they must take
// it from here rather than from the field, or the read races the replacement.
func (app *Application) CurrentSettings() *database.Setting {
	app.settingsMu.RLock()
	defer app.settingsMu.RUnlock()

	return app.settings
}

// SetSettings replaces the in-memory settings copy.
func (app *Application) SetSettings(settings *database.Setting) {
	app.settingsMu.Lock()
	defer app.settingsMu.Unlock()

	app.settings = settings
}

// withSettingsWrite runs a read-modify-write against the settings row while
// holding the write lock, so two concurrent admin updates cannot both read the
// same row and then each write back the other's omitted fields. fn receives the
// current row and returns the updated one; the cache is refreshed on success.
//
// Anything that touches the filesystem or the network must be validated before
// calling this -- the lock is held for the whole callback.
func (app *Application) withSettingsWrite(fn func(current database.Setting) (database.Setting, error)) (database.Setting, error) {
	app.settingsMu.Lock()
	defer app.settingsMu.Unlock()

	// Reads the field directly: the write lock is already held, and RLock on a
	// sync.RWMutex is not reentrant.
	updated, err := fn(*app.settings)
	if err != nil {
		return database.Setting{}, err
	}

	app.settings = &updated

	return updated, nil
}

func (app *Application) GetSettings(w http.ResponseWriter, _ *http.Request) {
	settings := *app.CurrentSettings()

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: mapLibrarySettingsResponse(settings)})
}

func (app *Application) GetGeneralSettings(w http.ResponseWriter, _ *http.Request) {
	settings := *app.CurrentSettings()

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Data: map[string]any{"settings": mapGeneralSettingsResponse(settings)},
	})
}

func (app *Application) UpdateGeneralSettings(w http.ResponseWriter, r *http.Request) {
	var req updateGeneralSettingsRequest
	err := helpers.ReadJSON(w, r, &req)
	if err != nil {
		helpers.ErrorJSON(w, errors.New(invalidRequestBodyMessage), http.StatusBadRequest)
		return
	}

	missing := req.missingField()
	if missing != "" {
		helpers.ErrorJSON(w, errors.New(missing+" is required"), http.StatusBadRequest)
		return
	}

	values := req.values()

	if !isOptionalHTTPURL(values.jellyfinBaseURL) {
		helpers.ErrorJSON(w, errors.New("jellyfin base URL must be a valid http or https URL"), http.StatusBadRequest)
		return
	}

	if !isOptionalHTTPURL(values.immichBaseURL) {
		helpers.ErrorJSON(w, errors.New("immich base URL must be a valid http or https URL"), http.StatusBadRequest)
		return
	}

	if values.staticDir == "" {
		helpers.ErrorJSON(w, errors.New("static directory is required"), http.StatusBadRequest)
		return
	}

	if values.transcodeDir == "" {
		helpers.ErrorJSON(w, errors.New("transcode directory is required"), http.StatusBadRequest)
		return
	}

	_, err = helpers.GetOrCreateDir(values.staticDir)
	if err != nil {
		app.Logger.Error("failed to validate static directory", "error", err, "path", values.staticDir)
		helpers.ErrorJSON(w, errors.New("static directory is not accessible"), http.StatusBadRequest)
		return
	}

	_, err = helpers.GetOrCreateDir(values.transcodeDir)
	if err != nil {
		app.Logger.Error("failed to validate transcode directory", "error", err, "path", values.transcodeDir)
		helpers.ErrorJSON(w, errors.New("transcode directory is not accessible"), http.StatusBadRequest)
		return
	}

	// The directory validation above touches the filesystem, so it stays
	// outside the lock; only the read-compare-write runs under it.
	var restartRequired bool
	updatedSettings, err := app.withSettingsWrite(func(current database.Setting) (database.Setting, error) {
		updated, err := app.Queries.UpdateGeneralSettings(r.Context(), database.UpdateGeneralSettingsParams{
			TmdbKey:             helpers.NullString(values.tmdbKey),
			ImmichBaseUrl:       helpers.NullString(values.immichBaseURL),
			ImmichApiKey:        helpers.NullString(values.immichApiKey),
			JellyfinBaseUrl:     helpers.NullString(values.jellyfinBaseURL),
			JellyfinApiKey:      helpers.NullString(values.jellyfinApiKey),
			SpotifyClientID:     helpers.NullString(values.spotifyClientID),
			SpotifyClientSecret: helpers.NullString(values.spotifyClientSecret),
			EnableWatcher:       values.enableWatcher,
			DownloadImages:      values.downloadImages,
			StaticDir:           values.staticDir,
			TranscodeDir:        values.transcodeDir,
		})
		if err != nil {
			return database.Setting{}, err
		}

		restartRequired = generalSettingsRestartRequired(&current, updated)

		return updated, nil
	})
	if err != nil {
		app.serverError(w, err, "failed to update general settings", "failed to update settings")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Settings updated",
		Data: map[string]any{
			"settings":         mapGeneralSettingsResponse(updatedSettings),
			"restart_required": restartRequired,
		},
	})
}

// generalSettingsRestartRequired reports whether a change only takes effect
// after a restart: the TMDB and Spotify clients are built once at startup.
// The static and transcode directories are read live, and the remaining
// integration values are stored without a server feature that reads them.
func generalSettingsRestartRequired(previous *database.Setting, next database.Setting) bool {
	return previous.TmdbKey != next.TmdbKey ||
		previous.SpotifyClientID != next.SpotifyClientID ||
		previous.SpotifyClientSecret != next.SpotifyClientSecret
}

func (app *Application) UpdateLibrarySettings(w http.ResponseWriter, r *http.Request) {
	var req librarySettings
	err := helpers.ReadJSON(w, r, &req)
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

	// The directory validation above touches the filesystem, so it stays
	// outside the lock; only the write and the cache refresh run under it.
	updatedSettings, err := app.withSettingsWrite(func(database.Setting) (database.Setting, error) {
		return app.Queries.UpdateLibrarySettings(r.Context(), database.UpdateLibrarySettingsParams{
			MoviesDir: moviesDir,
			ShowsDir:  showsDir,
			MusicDir:  musicDir,
		})
	})
	if err != nil {
		app.serverError(w, err, "failed to update library settings", "failed to update library settings")
		return
	}

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{
		Message: "Library settings updated",
		Data:    map[string]any{"settings": mapLibrarySettingsResponse(updatedSettings)},
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

	// The same check startup runs, which also opens the directory: a path
	// saved here that startup would reject is blanked at the next restart.
	err := helpers.ValidateDir(dir)
	if err != nil {
		return sql.NullString{}, err
	}

	return helpers.NullString(dir), nil
}

// scanLibrary carries the wording the scan endpoints and startup logs use for
// one library.
type scanLibrary struct {
	name      string // "movie"
	directory string // "movies"
	title     string // "Movie"
}

var (
	movieLibrary = scanLibrary{name: "movie", directory: "movies", title: "Movie"}
	showLibrary  = scanLibrary{name: "show", directory: "shows", title: "Show"}
	musicLibrary = scanLibrary{name: "music", directory: "music", title: "Music"}
)

func (app *Application) TriggerMusicScan(w http.ResponseWriter, r *http.Request) {
	app.triggerScan(w, musicLibrary, app.MusicScanner.Start())
}

func (app *Application) TriggerMovieScan(w http.ResponseWriter, r *http.Request) {
	app.triggerScan(w, movieLibrary, app.MovieScanner.Start())
}

func (app *Application) TriggerShowScan(w http.ResponseWriter, r *http.Request) {
	app.triggerScan(w, showLibrary, app.ShowScanner.Start())
}

func (app *Application) triggerScan(w http.ResponseWriter, library scanLibrary, result scanner.StartResult) {
	switch result.Status {
	case scanner.StartNotConfigured:
		helpers.ErrorJSON(w, fmt.Errorf("%s directory is not configured", library.directory))
		return
	case scanner.StartAlreadyRunning:
		helpers.ErrorJSON(w, fmt.Errorf("%s library scan is already in progress", library.name), http.StatusConflict)
		return
	}

	app.Logger.Info(library.name+" library scan triggered via API", "path", result.Directory)

	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Message: library.title + " library scan started"})
}

func (app *Application) GetMovieScanStatus(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: app.MovieScanner.Status()})
}

func (app *Application) GetShowScanStatus(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: app.ShowScanner.Status()})
}

func (app *Application) GetMusicScanStatus(w http.ResponseWriter, r *http.Request) {
	helpers.WriteJSON(w, http.StatusOK, helpers.JSONResponse{Data: app.MusicScanner.Status()})
}
