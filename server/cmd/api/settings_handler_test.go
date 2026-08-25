package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner/movie"

	"github.com/go-chi/chi/v5"
)

func generalSettingsBody(staticDir string) string {
	transcodeDir := filepath.Join(filepath.Dir(staticDir), "transcode")
	return fmt.Sprintf(`{
		"tmdb_key": "tmdb-key",
		"jellyfin_base_url": "https://jellyfin.local:8096/base",
		"jellyfin_api_key": "jellyfin-api-key",
		"immich_base_url": "http://immich.local:2283",
		"immich_api_key": "immich-api-key",
		"spotify_client_id": "spotify-id",
		"spotify_client_secret": "spotify-secret",
		"enable_watcher": true,
		"download_images": true,
		"static_dir": %q,
		"transcode_dir": %q
	}`, staticDir, transcodeDir)
}

func performUpdateGeneralSettings(app *Application, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/api/settings/general", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.UpdateGeneralSettings(w, req)

	return w
}

func TestUpdateGeneralSettings_UpdatesDatabaseAndApplicationSettings(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	staticDir := filepath.Join(t.TempDir(), "static")
	w := performUpdateGeneralSettings(app, generalSettingsBody(staticDir))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}

	if settings.TmdbKey.String != "tmdb-key" || !settings.TmdbKey.Valid {
		t.Fatalf("expected TMDB key to be saved, got %q valid=%v", settings.TmdbKey.String, settings.TmdbKey.Valid)
	}
	if settings.JellyfinBaseUrl.String != "https://jellyfin.local:8096/base" || !settings.JellyfinBaseUrl.Valid {
		t.Fatalf("expected Jellyfin base URL to be saved, got %q valid=%v", settings.JellyfinBaseUrl.String, settings.JellyfinBaseUrl.Valid)
	}
	if settings.JellyfinApiKey.String != "jellyfin-api-key" || !settings.JellyfinApiKey.Valid {
		t.Fatalf("expected Jellyfin API key to be saved, got %q valid=%v", settings.JellyfinApiKey.String, settings.JellyfinApiKey.Valid)
	}
	if settings.ImmichBaseUrl.String != "http://immich.local:2283" || !settings.ImmichBaseUrl.Valid {
		t.Fatalf("expected Immich base URL to be saved, got %q valid=%v", settings.ImmichBaseUrl.String, settings.ImmichBaseUrl.Valid)
	}
	if settings.ImmichApiKey.String != "immich-api-key" || !settings.ImmichApiKey.Valid {
		t.Fatalf("expected Immich API key to be saved, got %q valid=%v", settings.ImmichApiKey.String, settings.ImmichApiKey.Valid)
	}
	if !settings.EnableWatcher || !settings.DownloadImages {
		t.Fatal("expected boolean general settings to be enabled")
	}
	if settings.StaticDir != staticDir {
		t.Fatalf("expected static dir %q, got %q", staticDir, settings.StaticDir)
	}
	if settings.TranscodeDir != filepath.Join(filepath.Dir(staticDir), "transcode") {
		t.Fatalf("expected transcode dir to be saved, got %q", settings.TranscodeDir)
	}
	if app.settings.StaticDir != staticDir {
		t.Fatal("expected app.settings to reflect the saved general settings")
	}
	if app.settings.ImmichApiKey.String != "immich-api-key" || !app.settings.ImmichApiKey.Valid {
		t.Fatalf("expected app.settings Immich API key to be saved, got %q valid=%v", app.settings.ImmichApiKey.String, app.settings.ImmichApiKey.Valid)
	}
}

func TestSettingsHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()
	app.Wait = &sync.WaitGroup{}

	assertRequest := func(operationID string, req *http.Request, serve func(http.ResponseWriter, *http.Request), wantStatus int) {
		t.Helper()
		addOpenAPITestCookie(req)
		response := httptest.NewRecorder()
		serve(response, req)
		if response.Code != wantStatus {
			t.Fatalf("%s status = %d, want %d, body = %s", operationID, response.Code, wantStatus, response.Body.String())
		}
		assertOpenAPIExchange(t, operationID, req, response)
	}

	assertRequest("getSettings", httptest.NewRequest(http.MethodGet, "/api/settings", nil), app.GetSettings, http.StatusOK)
	assertRequest("getGeneralSettings", httptest.NewRequest(http.MethodGet, "/api/settings/general", nil), app.GetGeneralSettings, http.StatusOK)

	staticDir := filepath.Join(t.TempDir(), "static")
	generalReq := newOpenAPIJSONRequest(http.MethodPut, "/api/settings/general", generalSettingsBody(staticDir))
	assertRequest("updateGeneralSettings", generalReq, app.UpdateGeneralSettings, http.StatusOK)

	mediaRoot := t.TempDir()
	moviesDir := filepath.Join(mediaRoot, "movies")
	showsDir := filepath.Join(mediaRoot, "shows")
	musicDir := filepath.Join(mediaRoot, "music")
	for _, dir := range []string{moviesDir, showsDir, musicDir} {
		_, err := helpers.GetOrCreateDir(dir)
		if err != nil {
			t.Fatalf("create media directory: %v", err)
		}
	}
	libraryBody := fmt.Sprintf(`{"movies_dir":%q,"shows_dir":%q,"music_dir":%q}`, moviesDir, showsDir, musicDir)
	libraryReq := newOpenAPIJSONRequest(http.MethodPut, "/api/settings/libraries", libraryBody)
	assertRequest("updateLibrarySettings", libraryReq, app.UpdateLibrarySettings, http.StatusOK)

	assertRequest("triggerMusicScan", httptest.NewRequest(http.MethodPost, "/api/settings/scan/music", nil), app.TriggerMusicScan, http.StatusOK)
	assertRequest("triggerMovieScan", httptest.NewRequest(http.MethodPost, "/api/settings/scan/movies", nil), app.TriggerMovieScan, http.StatusOK)
	app.Wait.Wait()
}

func TestUpdateGeneralSettings_RejectsInvalidIntegrationBaseURLs(t *testing.T) {
	cases := []struct {
		name string
		old  string
		new  string
	}{
		{
			name: "jellyfin invalid scheme",
			old:  `"jellyfin_base_url": "https://jellyfin.local:8096/base"`,
			new:  `"jellyfin_base_url": "ftp://jellyfin.local"`,
		},
		{
			name: "jellyfin missing host",
			old:  `"jellyfin_base_url": "https://jellyfin.local:8096/base"`,
			new:  `"jellyfin_base_url": "https:///jellyfin"`,
		},
		{
			name: "immich missing scheme",
			old:  `"immich_base_url": "http://immich.local:2283"`,
			new:  `"immich_base_url": "immich.local:2283"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupSettingsTestApp(t)
			defer app.DB.Close()

			staticDir := filepath.Join(t.TempDir(), "static")
			body := strings.Replace(generalSettingsBody(staticDir), tc.old, tc.new, 1)
			w := performUpdateGeneralSettings(app, body)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestUpdateGeneralSettings_ClearsOptionalStringSettings(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	staticDir := filepath.Join(t.TempDir(), "static")
	w := performUpdateGeneralSettings(app, generalSettingsBody(staticDir))
	if w.Code != http.StatusOK {
		t.Fatalf("expected setup update 200, got %d: %s", w.Code, w.Body.String())
	}

	clearBody := fmt.Sprintf(`{
		"tmdb_key": "",
		"jellyfin_base_url": "",
		"jellyfin_api_key": "",
		"immich_base_url": "",
		"immich_api_key": "",
		"spotify_client_id": "",
		"spotify_client_secret": "",
		"enable_watcher": false,
		"download_images": false,
		"static_dir": %q,
		"transcode_dir": %q
	}`, staticDir, filepath.Join(filepath.Dir(staticDir), "transcode"))
	w = performUpdateGeneralSettings(app, clearBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected clear update 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings after clear: %v", err)
	}
	if settings.TmdbKey.Valid || settings.JellyfinApiKey.Valid ||
		settings.JellyfinBaseUrl.Valid || settings.ImmichBaseUrl.Valid ||
		settings.ImmichApiKey.Valid || settings.SpotifyClientID.Valid ||
		settings.SpotifyClientSecret.Valid {
		t.Fatal("expected optional string settings to be cleared")
	}
}

func TestUpdateGeneralSettings_RejectsEmptyRequiredDirectories(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	w := performUpdateGeneralSettings(app, generalSettingsBody(""))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func performUpdateLibrarySettings(app *Application, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/api/settings/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.UpdateLibrarySettings(w, req)

	return w
}

func TestUpdateLibrarySettings_UpdatesMediaDirectories(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	root := t.TempDir()
	moviesDir := filepath.Join(root, "movies")
	showsDir := filepath.Join(root, "shows")
	musicDir := filepath.Join(root, "music")
	for _, dir := range []string{moviesDir, showsDir, musicDir} {
		if _, err := helpers.GetOrCreateDir(dir); err != nil {
			t.Fatalf("create media dir %s: %v", dir, err)
		}
	}

	body := fmt.Sprintf(`{
		"movies_dir": %q,
		"shows_dir": %q,
		"music_dir": %q
	}`, moviesDir, showsDir, musicDir)
	w := performUpdateLibrarySettings(app, body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}

	if settings.MoviesDir.String != moviesDir || !settings.MoviesDir.Valid {
		t.Fatalf("expected movies dir %q, got %q valid=%v", moviesDir, settings.MoviesDir.String, settings.MoviesDir.Valid)
	}
	if settings.ShowsDir.String != showsDir || !settings.ShowsDir.Valid {
		t.Fatalf("expected shows dir %q, got %q valid=%v", showsDir, settings.ShowsDir.String, settings.ShowsDir.Valid)
	}
	if settings.MusicDir.String != musicDir || !settings.MusicDir.Valid {
		t.Fatalf("expected music dir %q, got %q valid=%v", musicDir, settings.MusicDir.String, settings.MusicDir.Valid)
	}
}

func TestUpdateLibrarySettings_ClearsMediaDirectories(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	w := performUpdateLibrarySettings(app, `{
		"movies_dir": "",
		"shows_dir": null,
		"music_dir": ""
	}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings after update: %v", err)
	}

	if settings.MoviesDir.Valid || settings.ShowsDir.Valid || settings.MusicDir.Valid {
		t.Fatal("expected media directories to be cleared")
	}
}

func TestUpdateLibrarySettings_RejectsMissingMediaDirectory(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	body := fmt.Sprintf(`{
		"movies_dir": %q,
		"shows_dir": null,
		"music_dir": null
	}`, filepath.Join(t.TempDir(), "missing"))
	w := performUpdateLibrarySettings(app, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTriggerMusicScanRejectsAlreadyRunningScan(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()
	current := *app.CurrentSettings()
	current.MusicDir = sql.NullString{String: t.TempDir(), Valid: true}
	app.SetSettings(&current)

	musicScanGuard.Finish()
	if !musicScanGuard.TryBegin() {
		t.Fatal("failed to acquire music scan guard")
	}
	defer musicScanGuard.Finish()

	req := httptest.NewRequest(http.MethodPost, "/api/scan/music", nil)
	w := httptest.NewRecorder()

	app.TriggerMusicScan(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTriggerMovieScanRejectsAlreadyRunningScan(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()
	current := *app.CurrentSettings()
	current.MoviesDir = sql.NullString{String: t.TempDir(), Valid: true}
	app.SetSettings(&current)

	app.MovieScanner = movieStartResultStub{result: movie.StartResult{Status: movie.StartAlreadyRunning}}

	req := httptest.NewRequest(http.MethodPost, "/api/scan/movies", nil)
	w := httptest.NewRecorder()

	app.TriggerMovieScan(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTriggerMovieScanMapsStartStatusesToAdminResponses(t *testing.T) {
	tests := []struct {
		name       string
		result     movie.StartResult
		wantStatus int
	}{
		{"started", movie.StartResult{Directory: "/movies", Status: movie.StartStarted}, http.StatusOK},
		{"not configured", movie.StartResult{Status: movie.StartNotConfigured}, http.StatusInternalServerError},
		{"already running", movie.StartResult{Status: movie.StartAlreadyRunning}, http.StatusConflict},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := setupSettingsTestApp(t)
			defer app.DB.Close()
			app.MovieScanner = movieStartResultStub{result: tc.result}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/settings/scan/movies", nil)
			app.TriggerMovieScan(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

type movieStartResultStub struct {
	result movie.StartResult
}

func (s movieStartResultStub) Start() movie.StartResult {
	return s.result
}

func mountGeneralSettingsRouter(app *Application, userID int64) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), cookieUserID, userID)
			next.ServeHTTP(w, r)
		})
	})
	r.With(app.RequireAdmin).Put("/api/settings/general", app.UpdateGeneralSettings)

	return app.SessionManager.LoadAndSave(r)
}

func TestUpdateGeneralSettings_RejectsNonAdminUser(t *testing.T) {
	app := setupSettingsTestApp(t)
	defer app.DB.Close()

	user, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     "Regular User",
		Email:    "regular@example.com",
		Password: "hashed",
		IsAdmin:  false,
		Avatar:   sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := mountGeneralSettingsRouter(app, user.ID)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/general", strings.NewReader(generalSettingsBody(
		filepath.Join(t.TempDir(), "static"),
	)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
