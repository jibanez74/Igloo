package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

func generalSettingsBody(staticDir string) string {
	return generalSettingsBodyWithWatcher(staticDir, true)
}

func generalSettingsBodyWithWatcher(staticDir string, watcher bool) string {
	transcodeDir := filepath.Join(filepath.Dir(staticDir), "transcode")
	return fmt.Sprintf(`{
		"tmdb_key": "tmdb-key",
		"spotify_client_id": "spotify-id",
		"spotify_client_secret": "spotify-secret",
		"enable_apple_lookups": false,
		"enable_watcher": %t,
		"download_images": true,
		"static_dir": %q,
		"transcode_dir": %q
	}`, watcher, staticDir, transcodeDir)
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
	if settings.SpotifyClientID.String != "spotify-id" || !settings.SpotifyClientID.Valid {
		t.Fatalf("expected Spotify client ID to be saved, got %q valid=%v", settings.SpotifyClientID.String, settings.SpotifyClientID.Valid)
	}
	if settings.SpotifyClientSecret.String != "spotify-secret" || !settings.SpotifyClientSecret.Valid {
		t.Fatalf("expected Spotify client secret to be saved, got %q valid=%v", settings.SpotifyClientSecret.String, settings.SpotifyClientSecret.Valid)
	}
	if !settings.EnableWatcher || !settings.DownloadImages {
		t.Fatal("expected boolean general settings to be enabled")
	}
	if settings.EnableAppleLookups || app.CurrentSettings().EnableAppleLookups {
		t.Fatal("expected Apple lookups to be turned off in the database and the application settings")
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
	if app.settings.SpotifyClientSecret.String != "spotify-secret" || !app.settings.SpotifyClientSecret.Valid {
		t.Fatalf("expected app.settings Spotify client secret to be saved, got %q valid=%v", app.settings.SpotifyClientSecret.String, app.settings.SpotifyClientSecret.Valid)
	}
}

func TestGeneralSettingsRestartRequired_OnlyForStartupClients(t *testing.T) {
	previous := database.Setting{
		TmdbKey:      sql.NullString{String: "tmdb-key", Valid: true},
		StaticDir:    "/srv/igloo/static",
		TranscodeDir: "/srv/igloo/transcode",
	}
	tests := []struct {
		name   string
		change func(*database.Setting)
		want   bool
	}{
		{"tmdb key", func(s *database.Setting) { s.TmdbKey.String = "rotated" }, true},
		{"spotify client id", func(s *database.Setting) { s.SpotifyClientID = sql.NullString{String: "id", Valid: true} }, true},
		{"spotify client secret", func(s *database.Setting) { s.SpotifyClientSecret = sql.NullString{String: "secret", Valid: true} }, true},
		{"static dir", func(s *database.Setting) { s.StaticDir = "/elsewhere/static" }, false},
		{"transcode dir", func(s *database.Setting) { s.TranscodeDir = "/elsewhere/transcode" }, false},
		{"download images flag", func(s *database.Setting) { s.DownloadImages = true }, false},
		{"watcher flag", func(s *database.Setting) { s.EnableWatcher = true }, false},
		{"apple lookups flag", func(s *database.Setting) { s.EnableAppleLookups = true }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := previous
			tt.change(&next)
			if got := generalSettingsRestartRequired(&previous, next); got != tt.want {
				t.Fatalf("restart required = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSettingsHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)
	handler := authenticatedRouter(t, app, admin.ID)

	serveOpenAPIExchange(t, handler, "getSettings", httptest.NewRequest(http.MethodGet, "/api/settings", nil), http.StatusOK)
	serveOpenAPIExchange(t, handler, "getGeneralSettings", httptest.NewRequest(http.MethodGet, "/api/settings/general", nil), http.StatusOK)

	staticDir := filepath.Join(t.TempDir(), "static")
	// The watcher stays off: on, it would walk the libraries saved below as
	// they arm, and the scan triggers here would find that walk running.
	generalReq := newOpenAPIJSONRequest(http.MethodPut, "/api/settings/general", generalSettingsBodyWithWatcher(staticDir, false))
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

func TestTriggerScan_QueuesBehindARunOfChanges(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)
	app.MovieScanner = movieStartFunc(func() scanner.StartResult {
		return scanner.StartResult{Status: scanner.StartQueued, Directory: "/library/movies"}
	})
	w := serveAs(t, app, admin.ID, http.MethodPost, "/api/settings/scan/movies", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Movie library scan queued") {
		t.Fatalf("queued scan: status %d body %s", w.Code, w.Body.String())
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
		"spotify_client_id": "",
		"spotify_client_secret": "",
		"enable_apple_lookups": true,
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
	if settings.TmdbKey.Valid || settings.SpotifyClientID.Valid || settings.SpotifyClientSecret.Valid {
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

func TestUpdateLibrarySettings_RejectsANullBody(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

	moviesDir := t.TempDir()
	body := fmt.Sprintf(`{"movies_dir": %q, "shows_dir": null, "music_dir": null}`, moviesDir)
	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/libraries", body)
	if w.Code != http.StatusOK {
		t.Fatalf("seed: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/libraries", "null")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings after rejected update: %v", err)
	}
	if !settings.MoviesDir.Valid || settings.MoviesDir.String != moviesDir {
		t.Fatalf("expected movies dir %q to survive, got %q valid=%v", moviesDir, settings.MoviesDir.String, settings.MoviesDir.Valid)
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

// Startup rejects a library directory it cannot open, so saving one must too;
// otherwise it saves with 200 and is blanked at the next restart.
func TestUpdateLibrarySettings_RejectsUnreadableMediaDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)

	dir := filepath.Join(t.TempDir(), "locked")
	err := os.Mkdir(dir, 0o000)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	body := fmt.Sprintf(`{"movies_dir": %q, "shows_dir": null, "music_dir": null}`, dir)
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

// The endpoint replaces the whole configuration, so a field the request leaves
// out used to read as blank or false and erase the stored value.
func TestUpdateGeneralSettings_RejectsMissingRequiredFields(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)
	handler := authenticatedRouter(t, app, admin.ID)

	staticDir := filepath.Join(t.TempDir(), "static")
	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", generalSettingsBody(staticDir))
	if w.Code != http.StatusOK {
		t.Fatalf("seed update status = %d, want 200: %s", w.Code, w.Body.String())
	}
	seeded, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var complete map[string]any
	err = json.Unmarshal([]byte(generalSettingsBody(staticDir)), &complete)
	if err != nil {
		t.Fatal(err)
	}

	for field := range complete {
		for _, variant := range []string{"omitted", "null"} {
			t.Run(field+"/"+variant, func(t *testing.T) {
				body := make(map[string]any, len(complete))
				maps.Copy(body, complete)
				if variant == "null" {
					body[field] = nil
				} else {
					delete(body, field)
				}
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}

				req := newOpenAPIJSONRequest(http.MethodPut, "/api/settings/general", string(encoded))
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				assertOpenAPIResponse(t, "updateGeneralSettings", req, w)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
				}
				if !strings.Contains(w.Body.String(), field+" is required") {
					t.Fatalf("body = %s, want it to name %s", w.Body.String(), field)
				}

				stored, err := app.Queries.GetSettings(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if stored != seeded {
					t.Fatalf("settings changed by a rejected request:\n got %+v\nwant %+v", stored, seeded)
				}
				if *app.CurrentSettings() != seeded {
					t.Fatal("in-memory settings changed by a rejected request")
				}
			})
		}
	}
}

// A save writes the database, whose one connection can be busy with a scanner
// batch. Readers, among them every poster and HLS start, keep the current copy
// meanwhile rather than waiting on the write.
func TestWithSettingsWrite_ReadersKeepTheCurrentCopyDuringTheWrite(t *testing.T) {
	app := setupTestApp(t)
	before := app.CurrentSettings()

	entered := make(chan struct{})
	release := make(chan struct{})
	written := make(chan error, 1)
	go func() {
		_, err := app.withSettingsWrite(func(current database.Setting) (database.Setting, error) {
			close(entered)
			<-release
			current.TranscodeDir = "/elsewhere"
			return current, nil
		})
		written <- err
	}()
	<-entered

	read := make(chan *database.Setting, 1)
	go func() {
		read <- app.CurrentSettings()
	}()
	select {
	case got := <-read:
		if got != before {
			close(release)
			t.Fatal("a reader saw a new copy before the write finished")
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("CurrentSettings waited for the settings write")
	}

	close(release)
	err := <-written
	if err != nil {
		t.Fatalf("withSettingsWrite: %v", err)
	}
	if app.CurrentSettings().TranscodeDir != "/elsewhere" {
		t.Fatalf("TranscodeDir = %q after the write, want /elsewhere", app.CurrentSettings().TranscodeDir)
	}
}
