package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func setupSettingsHTTPTestApp(t *testing.T) *Application {
	t.Helper()

	app := setupTestApp(t)
	app.InitSession()
	clearSettingsEnv(t)

	err := app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	return app
}

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
		"hardware_acceleration_device": "nvidia",
		"enable_watcher": true,
		"download_images": true,
		"static_dir": %q,
		"transcode_dir": %q,
		"server_upload_mbps": 25
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
	app := setupSettingsHTTPTestApp(t)
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
	if settings.HardwareAccelerationDevice.String != helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatalf("expected hardware device nvidia, got %q", settings.HardwareAccelerationDevice.String)
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
	if app.Settings == nil || app.Settings.StaticDir != staticDir {
		t.Fatal("expected app.Settings to reflect the saved general settings")
	}
	if app.Settings.ImmichApiKey.String != "immich-api-key" || !app.Settings.ImmichApiKey.Valid {
		t.Fatalf("expected app.Settings Immich API key to be saved, got %q valid=%v", app.Settings.ImmichApiKey.String, app.Settings.ImmichApiKey.Valid)
	}
	if !settings.ServerUploadMbps.Valid || settings.ServerUploadMbps.Float64 != 25 {
		t.Fatalf("expected server upload Mbps 25, got valid=%v value=%v", settings.ServerUploadMbps.Valid, settings.ServerUploadMbps.Float64)
	}
	if app.Settings.ServerUploadMbps.Float64 != 25 {
		t.Fatalf("expected app.Settings.ServerUploadMbps 25, got %v", app.Settings.ServerUploadMbps.Float64)
	}
}

func TestUpdateGeneralSettings_RejectsInvalidServerUploadMbps(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()

	staticDir := filepath.Join(t.TempDir(), "static")
	body := strings.Replace(generalSettingsBody(staticDir), `"server_upload_mbps": 25`, `"server_upload_mbps": -1`, 1)
	w := performUpdateGeneralSettings(app, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
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
			app := setupSettingsHTTPTestApp(t)
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

func TestUpdateGeneralSettings_ServerUploadMbpsBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		value      string
		wantStatus int
		wantStored float64
	}{
		{"zero rejected", "0", http.StatusBadRequest, 0},
		{"just above zero accepted", "0.1", http.StatusOK, 0.1},
		{"just below ceiling accepted", "99999.99", http.StatusOK, 99999.99},
		{"ceiling rejected", "100000", http.StatusBadRequest, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupSettingsHTTPTestApp(t)
			defer app.DB.Close()

			staticDir := filepath.Join(t.TempDir(), "static")
			body := strings.Replace(generalSettingsBody(staticDir), `"server_upload_mbps": 25`, `"server_upload_mbps": `+tc.value, 1)
			w := performUpdateGeneralSettings(app, body)

			if w.Code != tc.wantStatus {
				t.Fatalf("value=%s expected status %d, got %d: %s", tc.value, tc.wantStatus, w.Code, w.Body.String())
			}

			if tc.wantStatus == http.StatusOK {
				settings, err := app.Queries.GetSettings(context.Background())
				if err != nil {
					t.Fatalf("GetSettings: %v", err)
				}
				if !settings.ServerUploadMbps.Valid || settings.ServerUploadMbps.Float64 != tc.wantStored {
					t.Fatalf("expected stored server_upload_mbps %v, got valid=%v %v", tc.wantStored, settings.ServerUploadMbps.Valid, settings.ServerUploadMbps.Float64)
				}
			}
		})
	}
}

func TestUpdateGeneralSettings_ClearsServerUploadMbpsWhenNull(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()

	staticDir := filepath.Join(t.TempDir(), "static")
	w := performUpdateGeneralSettings(app, generalSettingsBody(staticDir))
	if w.Code != http.StatusOK {
		t.Fatalf("setup update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	clearBody := strings.Replace(generalSettingsBody(staticDir), `"server_upload_mbps": 25`, `"server_upload_mbps": null`, 1)
	w = performUpdateGeneralSettings(app, clearBody)
	if w.Code != http.StatusOK {
		t.Fatalf("clear update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings after clear: %v", err)
	}
	if settings.ServerUploadMbps.Valid {
		t.Fatalf("expected server upload Mbps to be cleared in DB, got %v", settings.ServerUploadMbps.Float64)
	}

	if app.Settings.ServerUploadMbps.Valid {
		t.Fatalf("expected app.Settings.ServerUploadMbps to be cleared, got %v", app.Settings.ServerUploadMbps.Float64)
	}
}

func TestUpdateGeneralSettings_ClearsOptionalStringSettings(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
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
		"hardware_acceleration_device": "cpu",
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

func TestUpdateGeneralSettings_RejectsInvalidHardwareDevice(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()

	staticDir := filepath.Join(t.TempDir(), "static")
	body := strings.Replace(generalSettingsBody(staticDir), `"nvidia"`, `"unsupported"`, 1)
	w := performUpdateGeneralSettings(app, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGeneralSettings_RejectsEmptyRequiredDirectories(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
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
	app := setupSettingsHTTPTestApp(t)
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
	app := setupSettingsHTTPTestApp(t)
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
	app := setupSettingsHTTPTestApp(t)
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
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()
	app.Settings.MusicDir = sql.NullString{String: t.TempDir(), Valid: true}

	finishMusicScan()
	if !tryBeginMusicScan() {
		t.Fatal("failed to acquire music scan guard")
	}
	defer finishMusicScan()

	req := httptest.NewRequest(http.MethodPost, "/api/scan/music", nil)
	w := httptest.NewRecorder()

	app.TriggerMusicScan(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTriggerMovieScanRejectsAlreadyRunningScan(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()
	app.Settings.MoviesDir = sql.NullString{String: t.TempDir(), Valid: true}

	finishMovieScan()
	if !tryBeginMovieScan() {
		t.Fatal("failed to acquire movie scan guard")
	}
	defer finishMovieScan()

	req := httptest.NewRequest(http.MethodPost, "/api/scan/movies", nil)
	w := httptest.NewRecorder()

	app.TriggerMovieScan(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func mountGeneralSettingsRouter(app *Application, userID int64) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
			next.ServeHTTP(w, r)
		})
	})
	r.With(app.RequireAdmin).Put("/api/settings/general", app.UpdateGeneralSettings)

	return app.SessionManager.LoadAndSave(r)
}

func TestUpdateGeneralSettings_RejectsNonAdminUser(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
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
