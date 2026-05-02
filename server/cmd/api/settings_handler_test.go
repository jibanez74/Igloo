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

func generalSettingsBody(staticDir, logsDir string) string {
	return fmt.Sprintf(`{
		"tmdb_key": "tmdb-key",
		"jellyfin_token": "jellyfin-token",
		"spotify_client_id": "spotify-id",
		"spotify_client_secret": "spotify-secret",
		"hardware_acceleration_device": "nvidia",
		"enable_logger": true,
		"enable_watcher": true,
		"download_images": true,
		"static_dir": %q,
		"logs_dir": %q
	}`, staticDir, logsDir)
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
	logsDir := filepath.Join(t.TempDir(), "logs")
	w := performUpdateGeneralSettings(app, generalSettingsBody(staticDir, logsDir))

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
	if settings.HardwareAccelerationDevice.String != helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA {
		t.Fatalf("expected hardware device nvidia, got %q", settings.HardwareAccelerationDevice.String)
	}
	if !settings.EnableLogger || !settings.EnableWatcher || !settings.DownloadImages {
		t.Fatal("expected boolean general settings to be enabled")
	}
	if settings.StaticDir != staticDir {
		t.Fatalf("expected static dir %q, got %q", staticDir, settings.StaticDir)
	}
	if settings.LogsDir != logsDir {
		t.Fatalf("expected logs dir %q, got %q", logsDir, settings.LogsDir)
	}
	if app.Settings == nil || app.Settings.StaticDir != staticDir {
		t.Fatal("expected app.Settings to reflect the saved general settings")
	}
}

func TestUpdateGeneralSettings_ClearsOptionalStringSettings(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()

	staticDir := filepath.Join(t.TempDir(), "static")
	logsDir := filepath.Join(t.TempDir(), "logs")
	w := performUpdateGeneralSettings(app, generalSettingsBody(staticDir, logsDir))
	if w.Code != http.StatusOK {
		t.Fatalf("expected setup update 200, got %d: %s", w.Code, w.Body.String())
	}

	clearBody := fmt.Sprintf(`{
		"tmdb_key": "",
		"jellyfin_token": "",
		"spotify_client_id": "",
		"spotify_client_secret": "",
		"hardware_acceleration_device": "cpu",
		"enable_logger": false,
		"enable_watcher": false,
		"download_images": false,
		"static_dir": %q,
		"logs_dir": %q
	}`, staticDir, logsDir)
	w = performUpdateGeneralSettings(app, clearBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected clear update 200, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings after clear: %v", err)
	}
	if settings.TmdbKey.Valid || settings.JellyfinToken.Valid ||
		settings.SpotifyClientID.Valid || settings.SpotifyClientSecret.Valid {
		t.Fatal("expected optional string settings to be cleared")
	}
}

func TestUpdateGeneralSettings_RejectsInvalidHardwareDevice(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()

	staticDir := filepath.Join(t.TempDir(), "static")
	logsDir := filepath.Join(t.TempDir(), "logs")
	body := strings.Replace(generalSettingsBody(staticDir, logsDir), `"nvidia"`, `"unsupported"`, 1)
	w := performUpdateGeneralSettings(app, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGeneralSettings_RejectsEmptyRequiredDirectories(t *testing.T) {
	app := setupSettingsHTTPTestApp(t)
	defer app.DB.Close()

	logsDir := filepath.Join(t.TempDir(), "logs")
	w := performUpdateGeneralSettings(app, generalSettingsBody("", logsDir))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
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
		filepath.Join(t.TempDir(), "logs"),
	)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
