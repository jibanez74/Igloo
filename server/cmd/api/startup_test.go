package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

func TestCleanupStaleHLSTempDirsUsesConfiguredTranscodeDir(t *testing.T) {
	transcodeDir := t.TempDir()
	staleDir, err := os.MkdirTemp(transcodeDir, "igloo-hls-*")
	if err != nil {
		t.Fatalf("create stale HLS dir: %v", err)
	}

	app := &Application{}
	setupTestLogger(t, app)

	cleanupStaleHLSTempDirs(app.Logger, transcodeDir)

	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Fatalf("expected stale HLS dir to be removed, stat err=%v", err)
	}
}

func TestCleanupStaleHLSTempDirsSkipsBlankDirectory(t *testing.T) {
	workingDir := t.TempDir()
	changeWorkingDirectory(t, workingDir)

	staleDir, err := os.MkdirTemp(workingDir, "igloo-hls-*")
	if err != nil {
		t.Fatalf("create stale HLS dir: %v", err)
	}

	app := &Application{}
	setupTestLogger(t, app)
	cleanupStaleHLSTempDirs(app.Logger, "")

	if _, err := os.Stat(staleDir); err != nil {
		t.Fatalf("expected directory in working directory to remain: %v", err)
	}
}

func TestInitLogger_CreatesRuntimeLogsDirWhenFileLogging(t *testing.T) {
	logsDir := filepath.Join(t.TempDir(), "logs")
	app := &Application{
		Config: RuntimeConfig{
			LogsDir:     logsDir,
			LogToStdout: false,
		},
	}

	err := app.InitLogger()
	if err != nil {
		t.Fatalf("InitLogger failed: %v", err)
	}
	if app.LoggerCloser != nil {
		defer app.LoggerCloser()
	}

	info, err := os.Stat(logsDir)
	if err != nil {
		t.Fatalf("expected logs directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", logsDir)
	}
}

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	os.Setenv("DB_PATH", dbPath)
	defer os.Unsetenv("DB_PATH")

	app := &Application{}
	setupTestLogger(t, app)

	err := app.InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer app.DB.Close()

	err = app.DB.Ping()
	if err != nil {
		t.Errorf("Database ping failed: %v", err)
	}

	var journalMode string
	err = app.DB.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Errorf("Failed to query journal mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("Expected journal_mode 'wal', got '%s'", journalMode)
	}

	var foreignKeys int
	err = app.DB.QueryRow("PRAGMA foreign_keys;").Scan(&foreignKeys)
	if err != nil {
		t.Errorf("Failed to query foreign_keys: %v", err)
	}

	if foreignKeys != 1 {
		t.Errorf("Expected foreign_keys to be 1, got %d", foreignKeys)
	}

	var busyTimeout int
	err = app.DB.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout)
	if err != nil {
		t.Errorf("Failed to query busy_timeout: %v", err)
	}

	if busyTimeout != 5000 {
		t.Errorf("Expected busy_timeout 5000, got %d", busyTimeout)
	}

	// 1 is NORMAL. Paired with WAL this drops the per-commit fsync, which the
	// scanners pay once per movie and per track.
	var synchronous int
	err = app.DB.QueryRow("PRAGMA synchronous;").Scan(&synchronous)
	if err != nil {
		t.Errorf("Failed to query synchronous: %v", err)
	}

	if synchronous != 1 {
		t.Errorf("Expected synchronous NORMAL (1), got %d", synchronous)
	}

	// 2 is MEMORY, which keeps the library listings' sorters off disk.
	var tempStore int
	err = app.DB.QueryRow("PRAGMA temp_store;").Scan(&tempStore)
	if err != nil {
		t.Errorf("Failed to query temp_store: %v", err)
	}

	if tempStore != 2 {
		t.Errorf("Expected temp_store MEMORY (2), got %d", tempStore)
	}

	stats := app.DB.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("Expected max open connections 1, got %d", stats.MaxOpenConnections)
	}
}

func TestRefreshQueryPlannerStats(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	app := &Application{DB: db}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO users (name, email, password) VALUES ('Stats', 'stats@example.com', 'hash')",
	)
	if err != nil {
		t.Fatalf("Failed to seed a user: %v", err)
	}

	err = app.RefreshQueryPlannerStats()
	if err != nil {
		t.Fatalf("RefreshQueryPlannerStats failed: %v", err)
	}

	// PRAGMA optimize creates sqlite_stat1 the first time it decides a table is
	// worth analyzing. Without it the planner costs every query on built-in guesses.
	var statsTable string
	err = db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'sqlite_stat1'",
	).Scan(&statsTable)
	if err != nil {
		t.Fatalf("Expected sqlite_stat1 to exist after refreshing statistics: %v", err)
	}
}

func TestInitDB_DefaultPath(t *testing.T) {
	tmpDir := t.TempDir()
	changeWorkingDirectory(t, tmpDir)
	t.Setenv(envDBPath, "")

	dbFile := filepath.Join(tmpDir, defaultDBPath)

	app := &Application{}
	setupTestLogger(t, app)

	err := app.InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer app.DB.Close()

	if _, statErr := os.Stat(dbFile); os.IsNotExist(statErr) {
		t.Errorf("Database file was not created at %s", dbFile)
	}
}

func TestInitDB_ReturnsPathForUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to directories without user write permissions")
	}

	tmpDir := t.TempDir()
	dbDir := filepath.Join(tmpDir, "db")
	if err := os.Mkdir(dbDir, 0o500); err != nil {
		t.Fatalf("failed to create database directory: %v", err)
	}
	defer os.Chmod(dbDir, 0o700)

	dbFile := filepath.Join(dbDir, "igloo.db")
	t.Setenv("DB_PATH", dbFile)

	app := &Application{}
	setupTestLogger(t, app)

	err := app.InitDB()
	if err == nil {
		app.DB.Close()
		t.Fatal("expected InitDB to fail for an unwritable database directory")
	}

	msg := err.Error()
	for _, want := range []string{"database directory is not writable", dbDir, dbFile} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected error %q to contain %q", msg, want)
		}
	}
}

func TestInitTables(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	app := &Application{DB: db}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	expectedTables := []string{
		"users",
		"settings",
		"musicians",
		"albums",
		"tracks",
		"genres",
		"musician_genres",
		"musician_albums",
		"track_genres",
		"sessions",
		"movie_watch_progress",
	}

	for _, tableName := range expectedTables {
		t.Run("Table_"+tableName, func(t *testing.T) {

			var name string

			err := db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
				tableName,
			).Scan(&name)

			if err != nil {
				t.Errorf("Table '%s' does not exist: %v", tableName, err)
			}
		})
	}
}

func TestInitTables_Idempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}
	defer db.Close()

	app := &Application{DB: db}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("First InitTables call failed: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO movies (id, title, file_path, file_name, size, container, mime_type, adult)
		VALUES (1, 'Moonrise', '/movies/moonrise.mkv', 'moonrise.mkv', 1, 'mkv', 'video/x-matroska', false);
		INSERT INTO shows (id, directory_path, local_name, name, overview)
		VALUES (1, '/shows/Nightfall', 'Nightfall', 'Nightfall', 'Dusk falls on a quiet town.');
		INSERT INTO musicians (id, name, sort_name) VALUES (1, 'Aurora', 'aurora');
		INSERT INTO albums (id, title, sort_title, musician) VALUES (1, 'Daylight', 'daylight', 'Aurora');
		INSERT INTO tracks (
			id, title, sort_title, file_path, file_name, container, mime_type, codec,
			size, track_index, duration, disc, channels, channel_layout, bit_rate, profile,
			album_id, musician_id
		) VALUES (
			1, 'Sunrise', 'sunrise', '/music/sunrise.flac', 'sunrise.flac', 'flac', 'audio/flac', 'flac',
			1, 1, 180, 1, '2', 'stereo', 1000, '', 1, 1
		);
		INSERT INTO genres (id, tag, genre_type) VALUES (1, 'Pop', 'music');
		INSERT INTO track_musicians (track_id, musician_id) VALUES (1, 1);
		INSERT INTO track_genres (track_id, genre_id) VALUES (1, 1);
		INSERT INTO album_genres (album_id, genre_id, source) VALUES (1, 1, 'spotify');
		INSERT INTO musician_genres (musician_id, genre_id, source) VALUES (1, 1, 'spotify');
		INSERT INTO music_artist_identity (identity_key, musician_id) VALUES ('aurora', 1);
		INSERT INTO music_album_identity (title_key, artist_key, album_id) VALUES ('daylight', 'aurora', 1);
		INSERT INTO music_credit_metadata (track_id, musician_id, sort_name) VALUES (1, 1, 'aurora');
		INSERT INTO music_spotify_matches (entity_type, entity_id, status) VALUES ('album', 1, 'unmatched');
	`)
	if err != nil {
		t.Fatalf("populate catalog: %v", err)
	}

	checks := []struct {
		name  string
		query string
		want  string
	}{
		{"movie", `SELECT id || ':' || title || ':' || file_path FROM movies`, "1:Moonrise:/movies/moonrise.mkv"},
		{"musician", `SELECT id || ':' || name || ':' || sort_name FROM musicians`, "1:Aurora:aurora"},
		{"album", `SELECT id || ':' || title || ':' || musician FROM albums`, "1:Daylight:Aurora"},
		{"track", `SELECT id || ':' || title || ':' || file_path || ':' || album_id || ':' || musician_id FROM tracks`, "1:Sunrise:/music/sunrise.flac:1:1"},
		{"genre", `SELECT id || ':' || tag || ':' || genre_type FROM genres`, "1:Pop:music"},
		{"track credit", `SELECT track_id || ':' || musician_id FROM track_musicians`, "1:1"},
		{"track genre", `SELECT track_id || ':' || genre_id FROM track_genres`, "1:1"},
		{"derived musician album", `SELECT musician_id || ':' || album_id FROM musician_albums`, "1:1"},
		{"musician genre provenance", `SELECT group_concat(source) FROM (SELECT source FROM musician_genres WHERE musician_id = 1 AND genre_id = 1 ORDER BY source)`, "local,spotify"},
		{"album genre provenance", `SELECT group_concat(source) FROM (SELECT source FROM album_genres WHERE album_id = 1 AND genre_id = 1 ORDER BY source)`, "local,spotify"},
		{"artist alias", `SELECT identity_key || ':' || musician_id FROM music_artist_identity`, "aurora:1"},
		{"album alias", `SELECT title_key || ':' || artist_key || ':' || album_id FROM music_album_identity`, "daylight:aurora:1"},
		{"sort contribution", `SELECT track_id || ':' || musician_id || ':' || sort_name FROM music_credit_metadata`, "1:1:aurora"},
		{"match cache", `SELECT entity_type || ':' || entity_id || ':' || status FROM music_spotify_matches`, "album:1:unmatched"},
		{"movie search", `SELECT group_concat(rowid) FROM movies_fts WHERE movies_fts MATCH 'moonrise'`, "1"},
		{"show search", `SELECT group_concat(rowid) FROM shows_fts WHERE shows_fts MATCH 'nightfall dusk'`, "1"},
		{"album search", `SELECT group_concat(rowid) FROM albums_fts WHERE albums_fts MATCH 'daylight aurora'`, "1"},
		{"musician search", `SELECT group_concat(rowid) FROM musicians_fts WHERE musicians_fts MATCH 'aurora'`, "1"},
		{"track search", `SELECT group_concat(rowid) FROM tracks_search_fts WHERE tracks_search_fts MATCH 'sunrise daylight aurora'`, "1"},
		{"vocabulary generations", `SELECT group_concat(vocab_table || ':' || generation) FROM (SELECT * FROM search_vocab_generations ORDER BY vocab_table)`, "albums_fts_vocab:1,movies_fts_vocab:1,musicians_fts_vocab:1,shows_fts_vocab:1,tracks_search_fts_vocab:1"},
		{"foreign key integrity", `SELECT COUNT(*) FROM pragma_foreign_key_check`, "0"},
	}

	assertPopulatedState := func(t *testing.T) {
		t.Helper()
		for _, check := range checks {
			t.Run(check.name, func(t *testing.T) {
				var got string
				err := db.QueryRow(check.query).Scan(&got)
				if err != nil {
					t.Fatalf("read populated state: %v", err)
				}
				if got != check.want {
					t.Errorf("got %q, want %q", got, check.want)
				}
			})
		}
	}

	t.Run("before reinitialization", assertPopulatedState)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("Second InitTables call failed (not idempotent): %v", err)
	}

	t.Run("after reinitialization", assertPopulatedState)
}

func TestInitSettings_CreatesDefaultSettings(t *testing.T) {
	app := setupTestApp(t)

	clearSettingsRow(t, app)
	clearRuntimeConfigEnv(t)
	config, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}
	app.Config = config
	ctx := context.Background()

	err = app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	if app.settings == nil {
		t.Fatal("Settings should not be nil after InitSettings")
	}

	if app.settings.StaticDir != defaultStaticDir {
		t.Errorf("Expected StaticDir %q, got %q", defaultStaticDir, app.settings.StaticDir)
	}
	if app.settings.TranscodeDir != defaultTranscodeDir {
		t.Errorf("Expected TranscodeDir %q, got %q", defaultTranscodeDir, app.settings.TranscodeDir)
	}

	if app.settings.HardwareAccelerationDevice.String != "cpu" {
		t.Errorf("Expected HardwareAccelerationDevice 'cpu', got '%s'", app.settings.HardwareAccelerationDevice.String)
	}
	if !app.settings.HardwareAccelerationDevice.Valid {
		t.Error("Expected HardwareAccelerationDevice to be valid")
	}

	if app.settings.EnableWatcher != false {
		t.Error("Expected EnableWatcher to be false by default")
	}
	if app.settings.DownloadImages != false {
		t.Error("Expected DownloadImages to be false by default")
	}

	if app.settings.TmdbKey.Valid {
		t.Error("Expected TmdbKey to be invalid when not set")
	}
	if app.settings.JellyfinApiKey.Valid {
		t.Error("Expected JellyfinApiKey to be invalid when not set")
	}
	if app.settings.MoviesDir.Valid {
		t.Errorf("Expected MoviesDir to be disabled by default, got %q", app.settings.MoviesDir.String)
	}
	if app.settings.ShowsDir.Valid {
		t.Errorf("Expected ShowsDir to be disabled by default, got %q", app.settings.ShowsDir.String)
	}
	if app.settings.MusicDir.Valid {
		t.Errorf("Expected MusicDir to be disabled by default, got %q", app.settings.MusicDir.String)
	}

	var settingsCount int
	err = app.DB.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&settingsCount)
	if err != nil {
		t.Fatalf("Failed to count settings rows: %v", err)
	}
	if settingsCount != 1 {
		t.Fatalf("Expected exactly one settings row, got %d", settingsCount)
	}
}

func TestInitSettings_UsesEnvVars(t *testing.T) {
	app := setupTestApp(t)

	clearSettingsRow(t, app)
	clearRuntimeConfigEnv(t)

	t.Setenv("TMDB_API_KEY", "test-tmdb-key")
	t.Setenv("JELLYFIN_API_KEY", "test-jellyfin-api-key")
	t.Setenv("HARDWARE_ACCELERATION_DEVICE", "nvidia")
	t.Setenv("ENABLE_WATCHER", "true")
	t.Setenv("DOWNLOAD_IMAGES", "true")
	t.Setenv("MOVIES_DIR", "/host/movies")
	t.Setenv("SHOWS_DIR", "/host/shows")
	t.Setenv("MUSIC_DIR", "/host/music")
	t.Setenv("STATIC_DIR", "/host/static")
	t.Setenv("TRANSCODE_DIR", "/host/transcode")
	config, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}
	app.Config = config

	ctx := context.Background()
	err = app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	if app.settings.TmdbKey.String != "test-tmdb-key" || !app.settings.TmdbKey.Valid {
		t.Errorf("Expected TmdbKey 'test-tmdb-key' (valid), got '%s' (valid=%v)", app.settings.TmdbKey.String, app.settings.TmdbKey.Valid)
	}
	if app.settings.JellyfinApiKey.String != "test-jellyfin-api-key" || !app.settings.JellyfinApiKey.Valid {
		t.Errorf("Expected JellyfinApiKey 'test-jellyfin-api-key' (valid), got '%s' (valid=%v)", app.settings.JellyfinApiKey.String, app.settings.JellyfinApiKey.Valid)
	}
	if app.settings.HardwareAccelerationDevice.String != "nvidia" || !app.settings.HardwareAccelerationDevice.Valid {
		t.Errorf("Expected HardwareAccelerationDevice 'nvidia' (valid), got '%s' (valid=%v)", app.settings.HardwareAccelerationDevice.String, app.settings.HardwareAccelerationDevice.Valid)
	}
	if app.settings.MoviesDir.String != "/host/movies" || !app.settings.MoviesDir.Valid {
		t.Errorf("Expected MoviesDir %q (valid), got %q (valid=%v)", "/host/movies", app.settings.MoviesDir.String, app.settings.MoviesDir.Valid)
	}
	if app.settings.ShowsDir.String != "/host/shows" || !app.settings.ShowsDir.Valid {
		t.Errorf("Expected ShowsDir %q (valid), got %q (valid=%v)", "/host/shows", app.settings.ShowsDir.String, app.settings.ShowsDir.Valid)
	}
	if app.settings.MusicDir.String != "/host/music" || !app.settings.MusicDir.Valid {
		t.Errorf("Expected MusicDir %q (valid), got %q (valid=%v)", "/host/music", app.settings.MusicDir.String, app.settings.MusicDir.Valid)
	}

	if app.settings.StaticDir != "/host/static" {
		t.Errorf("Expected StaticDir %q, got %q", "/host/static", app.settings.StaticDir)
	}
	if app.settings.TranscodeDir != "/host/transcode" {
		t.Errorf("Expected TranscodeDir %q, got %q", "/host/transcode", app.settings.TranscodeDir)
	}

	if app.settings.EnableWatcher != true {
		t.Error("Expected EnableWatcher to be true")
	}
	if app.settings.DownloadImages != true {
		t.Error("Expected DownloadImages to be true")
	}
}

func TestInitSettings_ExistingSettingsIgnoreConfigSeeds(t *testing.T) {
	app := setupTestApp(t)

	clearSettingsRow(t, app)

	const (
		existingMoviesDir = "/media/movies"
		existingShowsDir  = "/media/shows"
		existingMusicDir  = "/media/music"
	)

	params := database.CreateSettingsParams{
		TmdbKey:                    sql.NullString{String: "existing-key", Valid: true},
		JellyfinApiKey:             sql.NullString{String: "existing-jellyfin", Valid: true},
		SpotifyClientID:            sql.NullString{String: "existing-spotify-id", Valid: true},
		SpotifyClientSecret:        sql.NullString{String: "existing-spotify-secret", Valid: true},
		HardwareAccelerationDevice: sql.NullString{String: "cpu", Valid: true},
		EnableWatcher:              false,
		DownloadImages:             false,
		MoviesDir:                  sql.NullString{String: existingMoviesDir, Valid: true},
		ShowsDir:                   sql.NullString{String: existingShowsDir, Valid: true},
		MusicDir:                   sql.NullString{String: existingMusicDir, Valid: true},
		StaticDir:                  defaultStaticDir,
		TranscodeDir:               defaultTranscodeDir,
	}
	_, err := app.Queries.CreateSettings(context.Background(), params)
	if err != nil {
		t.Fatalf("Failed to create test settings: %v", err)
	}

	// Config carries the first-run seeds; with a row already present none of
	// them may reach the loaded settings.
	app.Config.TmdbAPIKey = "override-key"
	app.Config.JellyfinAPIKey = "override-jellyfin"
	app.Config.SpotifyClientID = "override-spotify-id"
	app.Config.SpotifyClientSecret = "override-spotify-secret"
	app.Config.HardwareAccelerationDevice = "nvidia"
	app.Config.EnableWatcher = true
	app.Config.DownloadImages = true
	app.Config.MoviesDir = "/override/movies"
	app.Config.ShowsDir = "/override/shows"
	app.Config.MusicDir = "/override/music"
	app.Config.StaticDir = "/override/static"
	app.Config.TranscodeDir = "/override/transcode"

	err = app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	if app.settings.TmdbKey.String != "existing-key" {
		t.Errorf("Expected existing tmdb key, got %q", app.settings.TmdbKey.String)
	}
	if app.settings.JellyfinApiKey.String != "existing-jellyfin" {
		t.Errorf("Expected existing jellyfin api key, got %q", app.settings.JellyfinApiKey.String)
	}
	if app.settings.SpotifyClientID.String != "existing-spotify-id" {
		t.Errorf("Expected existing spotify client id, got %q", app.settings.SpotifyClientID.String)
	}
	if app.settings.SpotifyClientSecret.String != "existing-spotify-secret" {
		t.Errorf("Expected existing spotify client secret, got %q", app.settings.SpotifyClientSecret.String)
	}
	if app.settings.HardwareAccelerationDevice.String != "cpu" {
		t.Errorf("Expected existing hardware mode, got %q", app.settings.HardwareAccelerationDevice.String)
	}
	if app.settings.EnableWatcher {
		t.Error("Expected existing EnableWatcher to remain false")
	}
	if app.settings.DownloadImages {
		t.Error("Expected existing DownloadImages to remain false")
	}
	if app.settings.MoviesDir.String != existingMoviesDir {
		t.Errorf("Expected MoviesDir to remain fixed at %q, got %q", existingMoviesDir, app.settings.MoviesDir.String)
	}
	if !app.settings.MoviesDir.Valid {
		t.Error("Expected MoviesDir.Valid to remain true")
	}
	if app.settings.ShowsDir.String != existingShowsDir {
		t.Errorf("Expected ShowsDir to remain fixed at %q, got %q", existingShowsDir, app.settings.ShowsDir.String)
	}
	if !app.settings.ShowsDir.Valid {
		t.Error("Expected ShowsDir.Valid to remain true")
	}
	if app.settings.MusicDir.String != existingMusicDir {
		t.Errorf("Expected MusicDir to remain fixed at %q, got %q", existingMusicDir, app.settings.MusicDir.String)
	}
	if !app.settings.MusicDir.Valid {
		t.Error("Expected MusicDir.Valid to remain true")
	}
	if app.settings.StaticDir != defaultStaticDir {
		t.Errorf("Expected StaticDir to remain fixed at %q, got %q", defaultStaticDir, app.settings.StaticDir)
	}
	if app.settings.TranscodeDir != defaultTranscodeDir {
		t.Errorf("Expected TranscodeDir to remain fixed at %q, got %q", defaultTranscodeDir, app.settings.TranscodeDir)
	}
}

func TestInitSettings_Idempotent(t *testing.T) {
	app := setupTestApp(t)

	ctx := context.Background()

	err := app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("First InitSettings call failed: %v", err)
	}

	firstSettingsID := app.settings.ID

	err = app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("Second InitSettings call failed: %v", err)
	}

	if app.settings.ID != firstSettingsID {
		t.Errorf("Expected same settings ID %d, got %d", firstSettingsID, app.settings.ID)
	}

	var settingsCount int
	err = app.DB.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&settingsCount)
	if err != nil {
		t.Fatalf("Failed to count settings rows: %v", err)
	}
	if settingsCount != 1 {
		t.Fatalf("Expected exactly one settings row after repeated InitSettings calls, got %d", settingsCount)
	}
}

func TestInitSession_CookieSecureFollowsConfig(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprintf("secure=%t", secure), func(t *testing.T) {
			app := setupTestApp(t)
			app.Config.SessionCookieSecure = secure
			app.InitSession()

			if app.SessionManager.Cookie.Secure != secure {
				t.Fatalf("cookie secure = %t, want %t", app.SessionManager.Cookie.Secure, secure)
			}
		})
	}
}

func TestInitDefaultUser_RequiresConfiguredPassword(t *testing.T) {
	app := setupTestApp(t)

	err := app.InitDefaultUser(context.Background())
	if err == nil {
		t.Fatal("expected missing bootstrap password to fail")
	}
	if !strings.Contains(err.Error(), envDefaultAdminPassword) {
		t.Fatalf("expected error to mention %s, got %v", envDefaultAdminPassword, err)
	}
}

func TestInitDefaultUser_UsesConfiguredCredentials(t *testing.T) {
	app := setupTestApp(t)
	app.Config.DefaultAdminName = "First Admin"
	app.Config.DefaultAdminEmail = "first-admin@example.com"
	app.Config.DefaultAdminPassword = "long-unique-bootstrap-password"

	err := app.InitDefaultUser(context.Background())
	if err != nil {
		t.Fatalf("InitDefaultUser failed: %v", err)
	}

	adminID, err := app.Queries.GetAdminUser(context.Background())
	if err != nil {
		t.Fatalf("GetAdminUser failed: %v", err)
	}
	admin, err := app.Queries.GetUser(context.Background(), adminID)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Name != app.Config.DefaultAdminName || admin.Email != app.Config.DefaultAdminEmail {
		t.Fatalf("admin credentials = (%q, %q), want (%q, %q)", admin.Name, admin.Email, app.Config.DefaultAdminName, app.Config.DefaultAdminEmail)
	}
}

func TestInitDirs(t *testing.T) {
	tmpDir := t.TempDir()
	app := setupTestApp(t)

	moviesDir := filepath.Join(tmpDir, "movies")
	if err := os.Mkdir(moviesDir, 0o755); err != nil {
		t.Fatalf("failed to create movies directory: %v", err)
	}

	missingShowsDir := filepath.Join(tmpDir, "shows")
	musicFile := filepath.Join(tmpDir, "music-file")
	if err := os.WriteFile(musicFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create music file: %v", err)
	}

	app.SetSettings(&database.Setting{
		StaticDir:    filepath.Join(tmpDir, "static"),
		TranscodeDir: filepath.Join(tmpDir, "transcode"),
		MoviesDir: sql.NullString{
			String: moviesDir,
			Valid:  true,
		},
		ShowsDir: sql.NullString{
			String: missingShowsDir,
			Valid:  true,
		},
		MusicDir: sql.NullString{
			String: musicFile,
			Valid:  true,
		},
	})

	err := app.InitDirs()
	if err != nil {
		t.Fatalf("InitDirs failed: %v", err)
	}

	for _, dir := range []string{
		app.settings.StaticDir,
		app.settings.TranscodeDir,
		filepath.Join(app.settings.StaticDir, "albums"),
		filepath.Join(app.settings.StaticDir, "musicians"),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}

	if !app.settings.MoviesDir.Valid || app.settings.MoviesDir.String != moviesDir {
		t.Errorf("expected existing movies directory to remain configured, got %q (valid=%v)", app.settings.MoviesDir.String, app.settings.MoviesDir.Valid)
	}
	if app.settings.ShowsDir.Valid {
		t.Errorf("expected missing shows directory to be disabled, got %q", app.settings.ShowsDir.String)
	}
	if app.settings.MusicDir.Valid {
		t.Errorf("expected non-directory music path to be disabled, got %q", app.settings.MusicDir.String)
	}
	if _, err := os.Stat(missingShowsDir); !os.IsNotExist(err) {
		t.Errorf("expected missing shows directory not to be created, stat err=%v", err)
	}
}
