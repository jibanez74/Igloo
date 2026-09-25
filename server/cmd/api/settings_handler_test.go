package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
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

func TestUpdateGeneralSettings_UpdatesDatabaseAndApplicationSettings(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

	staticDir := filepath.Join(t.TempDir(), "static")
	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", generalSettingsBody(staticDir))

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
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)
	handler := authenticatedRouter(t, app, admin.ID)

	serveOpenAPIExchange(t, handler, "getSettings", httptest.NewRequest(http.MethodGet, "/api/settings", nil), http.StatusOK)
	serveOpenAPIExchange(t, handler, "getGeneralSettings", httptest.NewRequest(http.MethodGet, "/api/settings/general", nil), http.StatusOK)

	staticDir := filepath.Join(t.TempDir(), "static")
	generalReq := newOpenAPIJSONRequest(http.MethodPut, "/api/settings/general", generalSettingsBody(staticDir))
	serveOpenAPIExchange(t, handler, "updateGeneralSettings", generalReq, http.StatusOK)

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
	serveOpenAPIExchange(t, handler, "updateLibrarySettings", libraryReq, http.StatusOK)

	serveOpenAPIExchange(t, handler, "triggerMusicScan", httptest.NewRequest(http.MethodPost, "/api/settings/scan/music", nil), http.StatusOK)
	serveOpenAPIExchange(t, handler, "getMusicScanStatus", httptest.NewRequest(http.MethodGet, "/api/settings/scan/music", nil), http.StatusOK)
	serveOpenAPIExchange(t, handler, "getMovieScanStatus", httptest.NewRequest(http.MethodGet, "/api/settings/scan/movies", nil), http.StatusOK)
	serveOpenAPIExchange(t, handler, "triggerMovieScan", httptest.NewRequest(http.MethodPost, "/api/settings/scan/movies", nil), http.StatusOK)
	serveOpenAPIExchange(t, handler, "triggerShowScan", httptest.NewRequest(http.MethodPost, "/api/settings/scan/shows", nil), http.StatusOK)
	serveOpenAPIExchange(t, handler, "getShowScanStatus", httptest.NewRequest(http.MethodGet, "/api/settings/scan/shows", nil), http.StatusOK)
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
			app := setupSessionTestApp(t)
			admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

			staticDir := filepath.Join(t.TempDir(), "static")
			body := strings.Replace(generalSettingsBody(staticDir), tc.old, tc.new, 1)
			w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", body)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestUpdateGeneralSettings_ClearsOptionalStringSettings(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

	staticDir := filepath.Join(t.TempDir(), "static")
	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", generalSettingsBody(staticDir))
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
	w = serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", clearBody)
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
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", generalSettingsBody(""))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateLibrarySettings_UpdatesMediaDirectories(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

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
	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/libraries", body)

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
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/libraries", `{
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
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

	body := fmt.Sprintf(`{
		"movies_dir": %q,
		"shows_dir": null,
		"music_dir": null
	}`, filepath.Join(t.TempDir(), "missing"))
	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/libraries", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateGeneralSettings_RejectsNonAdminUser(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "Regular User", "regular@example.com", false)

	handler := authenticatedRouter(t, app, user.ID)
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

func TestScanStatusAuthorization(t *testing.T) {
	for _, path := range []string{"/api/settings/scan/movies", "/api/settings/scan/music", "/api/settings/scan/shows"} {
		for _, role := range []string{"anonymous", "user", "admin"} {
			t.Run(path+"/"+role, func(t *testing.T) {
				app := setupSessionTestApp(t)
				app.InitRouter()
				request := httptest.NewRequest(http.MethodGet, path, nil)
				expected := http.StatusUnauthorized
				if role != "anonymous" {
					user := createTestUser(t, app, "Viewer", role+"@example.com", role == "admin")
					token := createTestDevice(t, app, user.ID, "Browser", "web")
					request.Header.Set("Authorization", "Bearer "+token)
					expected = http.StatusForbidden
					if role == "admin" {
						expected = http.StatusOK
					}
				}
				response := httptest.NewRecorder()
				app.Router.ServeHTTP(response, request)
				if response.Code != expected {
					t.Fatalf("status=%d want=%d body=%s", response.Code, expected, response.Body.String())
				}
			})
		}
	}
}

// The trigger endpoints map the scanner's start result to the admin response
// and sit behind RequireAdmin, for all three libraries alike.
func TestTriggerScan_MapsStartStatusesAndRequiresAdmin(t *testing.T) {
	kinds := []struct {
		name    string
		path    string
		install func(app *Application, start func() scanner.StartResult)
	}{
		{"movies", "/api/settings/scan/movies", func(app *Application, start func() scanner.StartResult) { app.MovieScanner = movieStartFunc(start) }},
		{"music", "/api/settings/scan/music", func(app *Application, start func() scanner.StartResult) { app.MusicScanner = musicStartFunc(start) }},
		{"shows", "/api/settings/scan/shows", func(app *Application, start func() scanner.StartResult) { app.ShowScanner = showStartFunc(start) }},
	}
	cases := []struct {
		name                 string
		authenticated, admin bool
		status               scanner.StartStatus
		want                 int
	}{
		{"unauthenticated", false, false, scanner.StartStarted, http.StatusUnauthorized},
		{"non-admin", true, false, scanner.StartStarted, http.StatusForbidden},
		{"started", true, true, scanner.StartStarted, http.StatusOK},
		{"already running", true, true, scanner.StartAlreadyRunning, http.StatusConflict},
		{"unconfigured", true, true, scanner.StartNotConfigured, http.StatusInternalServerError},
	}
	for _, kind := range kinds {
		for _, tc := range cases {
			t.Run(kind.name+"/"+tc.name, func(t *testing.T) {
				app := setupSessionTestApp(t)
				var userID int64
				if tc.authenticated {
					user := createTestUser(t, app, "User", "scan@example.com", tc.admin)
					userID = user.ID
				}
				calls := 0
				kind.install(app, func() scanner.StartResult {
					calls++
					return scanner.StartResult{Directory: "/media", Status: tc.status}
				})

				w := httptest.NewRecorder()
				authenticatedRouter(t, app, userID).ServeHTTP(w, httptest.NewRequest(http.MethodPost, kind.path, nil))
				if w.Code != tc.want {
					t.Fatalf("status %d want %d: %s", w.Code, tc.want, w.Body.String())
				}
				allowed := tc.authenticated && tc.admin
				if allowed && calls != 1 || !allowed && calls != 0 {
					t.Fatalf("scanner invoked %d times", calls)
				}
			})
		}
	}
}
