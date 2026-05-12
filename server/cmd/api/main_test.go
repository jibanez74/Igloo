package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	applogger "igloo/cmd/internal/logger"

	_ "github.com/mattn/go-sqlite3"
	cache "github.com/patrickmn/go-cache"
)

// setupTestLogger initializes a debug logger for tests.
func setupTestLogger(t *testing.T, app *Application) {
	t.Helper()

	logger, _, err := applogger.New(&applogger.LoggerConfig{
		Debug: true,
	})
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	app.Logger = logger
}

func TestInitDB(t *testing.T) {
	// Create a temporary directory for the test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Set the DB_PATH environment variable
	os.Setenv("DB_PATH", dbPath)
	defer os.Unsetenv("DB_PATH")

	app := &Application{}
	setupTestLogger(t, app)

	err := app.InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer app.DB.Close()

	// Verify database is open and responsive
	err = app.DB.Ping()
	if err != nil {
		t.Errorf("Database ping failed: %v", err)
	}

	// Verify WAL mode is enabled
	var journalMode string
	err = app.DB.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Errorf("Failed to query journal mode: %v", err)
	}

	if journalMode != "wal" {
		t.Errorf("Expected journal_mode 'wal', got '%s'", journalMode)
	}

	// Verify foreign keys are enabled
	var foreignKeys int
	err = app.DB.QueryRow("PRAGMA foreign_keys;").Scan(&foreignKeys)
	if err != nil {
		t.Errorf("Failed to query foreign_keys: %v", err)
	}

	if foreignKeys != 1 {
		t.Errorf("Expected foreign_keys to be 1, got %d", foreignKeys)
	}

	// Verify busy timeout is set
	var busyTimeout int
	err = app.DB.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout)
	if err != nil {
		t.Errorf("Failed to query busy_timeout: %v", err)
	}

	if busyTimeout != 5000 {
		t.Errorf("Expected busy_timeout 5000, got %d", busyTimeout)
	}
}

func TestInitDB_DefaultPath(t *testing.T) {
	// The production default (/config/igloo.db) is a Docker container path and
	// cannot be created in a test environment.  Verify instead that InitDB
	// honours DB_PATH and creates the database at the specified location.
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "igloo.db")
	t.Setenv("DB_PATH", dbFile)

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

func TestInitTables(t *testing.T) {
	// Create an in-memory database for testing
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

	// List of expected tables
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

func TestInitTables_Indexes(t *testing.T) {
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

	// List of expected indexes
	expectedIndexes := []string{
		"idx_user_name",
		"idx_musician_name",
		"idx_album_title",
		"idx_track_title",
		"idx_track_album",
		"idx_track_musician",
		"idx_genre_tag",
		"idx_musician_genres_musician",
		"idx_musician_genres_genre",
		"idx_musician_albums_musician",
		"idx_musician_albums_album",
		"idx_track_genres_track",
		"idx_track_genres_genre",
		"idx_sessions_expiry",
		"idx_movie_watch_progress_user_updated_at",
		"idx_settings_singleton",
	}

	for _, indexName := range expectedIndexes {
		t.Run("Index_"+indexName, func(t *testing.T) {

			var name string

			err := db.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
				indexName,
			).Scan(&name)

			if err != nil {
				t.Errorf("Index '%s' does not exist: %v", indexName, err)
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

	// Run InitTables twice - should not fail
	err = app.InitTables()
	if err != nil {
		t.Fatalf("First InitTables call failed: %v", err)
	}

	err = app.InitTables()
	if err != nil {
		t.Fatalf("Second InitTables call failed (not idempotent): %v", err)
	}
}

func TestInitTables_WatchRoomTrackConstraints(t *testing.T) {
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

	_, err = db.Exec(`
		INSERT INTO users (name, email, password)
		VALUES ('Owner', 'owner@example.com', 'hashed')
	`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult)
		VALUES ('Movie', '/tmp/movie.mkv', 'movie.mkv', 1, 'mkv', 'video/x-matroska', 0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO watch_rooms (owner_user_id, movie_id, playback_mode, audio_track)
		VALUES (1, 1, 'direct', -1)
	`)
	if err == nil {
		t.Fatal("expected negative audio_track insert to fail")
	}

	_, err = db.Exec(`
		INSERT INTO watch_rooms (owner_user_id, movie_id, playback_mode, audio_track, subtitle_track)
		VALUES (1, 1, 'direct', 0, -1)
	`)
	if err == nil {
		t.Fatal("expected negative subtitle_track insert to fail")
	}
}

func TestInitTables_SettingsSingleton(t *testing.T) {
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

	_, err = db.Exec(`
		INSERT INTO settings (tmdb_key, static_dir, logs_dir)
		VALUES ('first-key', 'first-static', 'first-logs')
	`)
	if err != nil {
		t.Fatalf("insert first settings row: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO settings (tmdb_key, static_dir, logs_dir)
		VALUES ('second-key', 'second-static', 'second-logs')
	`)
	if err == nil {
		t.Fatal("expected second settings insert to fail")
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&count)
	if err != nil {
		t.Fatalf("count settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one settings row, got %d", count)
	}
}

func TestInitTables_UsersSchema(t *testing.T) {
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

	// Test inserting a user to verify schema
	result, err := db.Exec(`
		INSERT INTO users (name, email, password) 
		VALUES ('Test User', 'test@example.com', 'hashedpassword')
	`)

	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get last insert id: %v", err)
	}

	if id != 1 {
		t.Errorf("Expected user id 1, got %d", id)
	}

	// Verify user was inserted correctly
	var name, email string
	var isAdmin bool

	err = db.QueryRow("SELECT name, email, is_admin FROM users WHERE id = ?", id).Scan(&name, &email, &isAdmin)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}

	if name != "Test User" {
		t.Errorf("Expected name 'Test User', got '%s'", name)
	}

	if email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", email)
	}

	if isAdmin != false {
		t.Errorf("Expected is_admin to be false by default")
	}
}

func TestInitTables_MovieMetadataLockColumns(t *testing.T) {
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

	rows, err := db.Query("PRAGMA table_info(movies)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(movies): %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int

		err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk)
		if err != nil {
			t.Fatalf("scan table_info row: %v", err)
		}
		columns[name] = true
	}

	expected := []string{
		"user_locked_title",
		"user_locked_tmdb_id",
		"user_locked_imdb_id",
		"user_locked_poster_path",
		"user_locked_backdrop_path",
		"user_locked_adult",
		"user_locked_language",
		"user_locked_year",
		"user_locked_release_date",
		"user_locked_overview",
		"user_locked_tag_line",
		"user_locked_certification",
		"user_locked_critic_rating",
		"user_locked_audience_rating",
		"user_locked_revenue",
		"user_locked_budget",
		"user_locked_run_time",
	}

	for _, columnName := range expected {
		if !columns[columnName] {
			t.Fatalf("expected movie metadata lock column %q to exist", columnName)
		}
	}
}

func TestInitTables_ForeignKeys(t *testing.T) {
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

	// Insert a musician first
	_, err = db.Exec(`
		INSERT INTO musicians (name, sort_name) 
		VALUES ('Test Artist', 'test artist')
	`)

	if err != nil {
		t.Fatalf("Failed to insert musician: %v", err)
	}

	// Insert an album
	_, err = db.Exec(`
		INSERT INTO albums (title, sort_title) 
		VALUES ('Test Album', 'test album')
	`)
	if err != nil {
		t.Fatalf("Failed to insert album: %v", err)
	}

	// Test many-to-many relationship
	_, err = db.Exec(`
		INSERT INTO musician_albums (musician_id, album_id) 
		VALUES (1, 1)
	`)

	if err != nil {
		t.Fatalf("Failed to insert musician_album relationship: %v", err)
	}

	// Verify foreign key constraint - try to insert invalid reference
	_, err = db.Exec(`
		INSERT INTO musician_albums (musician_id, album_id) 
		VALUES (999, 1)
	`)
	if err == nil {
		t.Error("Expected foreign key constraint violation, but insert succeeded")
	}
}

// Helper function to set up an Application with initialized DB, tables, queries, and logger.
func setupTestApp(t *testing.T) *Application {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	app := &Application{DB: db}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	app.Queries, err = database.Prepare(context.Background(), db)
	if err != nil {
		t.Fatalf("Failed to prepare queries: %v", err)
	}

	// Initialize in-memory caches without the production eviction callback
	// (no FFmpeg processes to kill in tests).
	app.HLSSessionCache = cache.New(helpers.HLS_SESSION_TTL, helpers.HLS_SESSION_CACHE_SWEEP)
	app.RemuxSafetyCache = cache.New(
		helpers.HLS_REMUX_SAFETY_CACHE_TTL,
		helpers.HLS_REMUX_SAFETY_CACHE_SWEEP,
	)
	app.SubtitleVTTCache = cache.New(helpers.SUBTITLE_CACHE_TTL, helpers.SUBTITLE_CACHE_CLEANUP)
	app.RoomHLSTombstone = cache.New(helpers.HLS_SESSION_TTL, helpers.HLS_SESSION_CACHE_SWEEP)
	app.WatchRoomHub = NewWatchRoomHub()

	return app
}

func clearSettingsEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TMDB_API_KEY",
		"JELLYFIN_TOKEN",
		"SPOTIFY_CLIENT_ID",
		"SPOTIFY_CLIENT_SECRET",
		"HARDWARE_ACCELERATION_DEVICE",
		"ENABLE_LOGGER",
		"ENABLE_WATCHER",
		"DOWNLOAD_IMAGES",
		"STATIC_DIR",
		"LOGS_DIR",
		"MOVIES_DIR",
		"SHOWS_DIR",
		"MUSIC_DIR",
	} {
		t.Setenv(key, "")
	}
}

func TestInitSettings_CreatesDefaultSettings(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	clearSettingsEnv(t)
	ctx := context.Background()

	err := app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	// Verify settings were created and stored
	if app.Settings == nil {
		t.Fatal("Settings should not be nil after InitSettings")
	}

	// Verify default values for required string fields
	if app.Settings.StaticDir != helpers.DEFAULT_STATIC_DIR {
		t.Errorf("Expected StaticDir %q, got %q", helpers.DEFAULT_STATIC_DIR, app.Settings.StaticDir)
	}
	if app.Settings.LogsDir != helpers.DEFAULT_LOGS_DIR {
		t.Errorf("Expected LogsDir %q, got %q", helpers.DEFAULT_LOGS_DIR, app.Settings.LogsDir)
	}

	// Verify default value for HardwareAccelerationDevice (defaults to "cpu")
	if app.Settings.HardwareAccelerationDevice.String != "cpu" {
		t.Errorf("Expected HardwareAccelerationDevice 'cpu', got '%s'", app.Settings.HardwareAccelerationDevice.String)
	}
	if !app.Settings.HardwareAccelerationDevice.Valid {
		t.Error("Expected HardwareAccelerationDevice to be valid")
	}

	// Verify boolean defaults (all false)
	if app.Settings.EnableLogger != false {
		t.Error("Expected EnableLogger to be false by default")
	}
	if app.Settings.EnableWatcher != false {
		t.Error("Expected EnableWatcher to be false by default")
	}
	if app.Settings.DownloadImages != false {
		t.Error("Expected DownloadImages to be false by default")
	}

	// Verify optional NullString fields are invalid when not set
	if app.Settings.TmdbKey.Valid {
		t.Error("Expected TmdbKey to be invalid when not set")
	}
	if app.Settings.JellyfinToken.Valid {
		t.Error("Expected JellyfinToken to be invalid when not set")
	}
	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String != helpers.DEFAULT_MOVIES_DIR {
		t.Errorf("Expected MoviesDir %q, got %q (valid=%v)", helpers.DEFAULT_MOVIES_DIR, app.Settings.MoviesDir.String, app.Settings.MoviesDir.Valid)
	}
	if !app.Settings.ShowsDir.Valid || app.Settings.ShowsDir.String != helpers.DEFAULT_SHOWS_DIR {
		t.Errorf("Expected ShowsDir %q, got %q (valid=%v)", helpers.DEFAULT_SHOWS_DIR, app.Settings.ShowsDir.String, app.Settings.ShowsDir.Valid)
	}
	if !app.Settings.MusicDir.Valid || app.Settings.MusicDir.String != helpers.DEFAULT_MUSIC_DIR {
		t.Errorf("Expected MusicDir %q, got %q (valid=%v)", helpers.DEFAULT_MUSIC_DIR, app.Settings.MusicDir.String, app.Settings.MusicDir.Valid)
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
	defer app.DB.Close()

	clearSettingsEnv(t)

	// Set all environment variables that InitSettings reads
	t.Setenv("TMDB_API_KEY", "test-tmdb-key")
	t.Setenv("JELLYFIN_TOKEN", "test-jellyfin-token")
	t.Setenv("HARDWARE_ACCELERATION_DEVICE", "nvidia")
	t.Setenv("ENABLE_LOGGER", "true")
	t.Setenv("ENABLE_WATCHER", "true")
	t.Setenv("DOWNLOAD_IMAGES", "true")

	ctx := context.Background()
	err := app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	// Verify NullString fields from env vars
	if app.Settings.TmdbKey.String != "test-tmdb-key" || !app.Settings.TmdbKey.Valid {
		t.Errorf("Expected TmdbKey 'test-tmdb-key' (valid), got '%s' (valid=%v)", app.Settings.TmdbKey.String, app.Settings.TmdbKey.Valid)
	}
	if app.Settings.JellyfinToken.String != "test-jellyfin-token" || !app.Settings.JellyfinToken.Valid {
		t.Errorf("Expected JellyfinToken 'test-jellyfin-token' (valid), got '%s' (valid=%v)", app.Settings.JellyfinToken.String, app.Settings.JellyfinToken.Valid)
	}
	if app.Settings.HardwareAccelerationDevice.String != "nvidia" || !app.Settings.HardwareAccelerationDevice.Valid {
		t.Errorf("Expected HardwareAccelerationDevice 'nvidia' (valid), got '%s' (valid=%v)", app.Settings.HardwareAccelerationDevice.String, app.Settings.HardwareAccelerationDevice.Valid)
	}
	if app.Settings.MoviesDir.String != helpers.DEFAULT_MOVIES_DIR || !app.Settings.MoviesDir.Valid {
		t.Errorf("Expected MoviesDir %q (valid), got %q (valid=%v)", helpers.DEFAULT_MOVIES_DIR, app.Settings.MoviesDir.String, app.Settings.MoviesDir.Valid)
	}
	if app.Settings.ShowsDir.String != helpers.DEFAULT_SHOWS_DIR || !app.Settings.ShowsDir.Valid {
		t.Errorf("Expected ShowsDir %q (valid), got %q (valid=%v)", helpers.DEFAULT_SHOWS_DIR, app.Settings.ShowsDir.String, app.Settings.ShowsDir.Valid)
	}
	if app.Settings.MusicDir.String != helpers.DEFAULT_MUSIC_DIR || !app.Settings.MusicDir.Valid {
		t.Errorf("Expected MusicDir %q (valid), got %q (valid=%v)", helpers.DEFAULT_MUSIC_DIR, app.Settings.MusicDir.String, app.Settings.MusicDir.Valid)
	}

	// Verify required string fields use fixed container defaults
	if app.Settings.StaticDir != helpers.DEFAULT_STATIC_DIR {
		t.Errorf("Expected StaticDir %q, got %q", helpers.DEFAULT_STATIC_DIR, app.Settings.StaticDir)
	}
	if app.Settings.LogsDir != helpers.DEFAULT_LOGS_DIR {
		t.Errorf("Expected LogsDir %q, got %q", helpers.DEFAULT_LOGS_DIR, app.Settings.LogsDir)
	}

	// Verify boolean fields from env vars
	if app.Settings.EnableLogger != true {
		t.Error("Expected EnableLogger to be true")
	}
	if app.Settings.EnableWatcher != true {
		t.Error("Expected EnableWatcher to be true")
	}
	if app.Settings.DownloadImages != true {
		t.Error("Expected DownloadImages to be true")
	}
}

func TestInitSettings_LoadsExistingSettings(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	params := database.CreateSettingsParams{
		TmdbKey:                    sql.NullString{String: "existing-key", Valid: true},
		JellyfinToken:              sql.NullString{Valid: false},
		HardwareAccelerationDevice: sql.NullString{String: "nvidia", Valid: true},
		EnableLogger:               true,
		EnableWatcher:              false,
		DownloadImages:             false,
		MoviesDir:                  sql.NullString{Valid: false},
		ShowsDir:                   sql.NullString{Valid: false},
		MusicDir:                   sql.NullString{Valid: false},
		StaticDir:                  "existing-static",
		LogsDir:                    "existing-logs",
	}
	_, err := app.Queries.CreateSettings(context.Background(), params)
	if err != nil {
		t.Fatalf("Failed to create test settings: %v", err)
	}

	// Now call InitSettings - it should load existing settings, not create new ones
	err = app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	// Verify the existing settings were loaded
	if app.Settings.TmdbKey.String != "existing-key" {
		t.Errorf("Expected TmdbKey 'existing-key', got '%s'", app.Settings.TmdbKey.String)
	}
	if app.Settings.StaticDir != "existing-static" {
		t.Errorf("Expected StaticDir 'existing-static', got '%s'", app.Settings.StaticDir)
	}
	if app.Settings.HardwareAccelerationDevice.String != "nvidia" {
		t.Errorf("Expected HardwareAccelerationDevice 'nvidia', got '%s'", app.Settings.HardwareAccelerationDevice.String)
	}
	if app.Settings.EnableLogger != true {
		t.Error("Expected EnableLogger to be true from existing settings")
	}
}

func TestInitSettings_ExistingSettingsAllowRuntimeOverrides(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	params := database.CreateSettingsParams{
		TmdbKey:                    sql.NullString{String: "existing-key", Valid: true},
		JellyfinToken:              sql.NullString{String: "existing-jellyfin", Valid: true},
		SpotifyClientID:            sql.NullString{String: "existing-spotify-id", Valid: true},
		SpotifyClientSecret:        sql.NullString{String: "existing-spotify-secret", Valid: true},
		HardwareAccelerationDevice: sql.NullString{String: "cpu", Valid: true},
		EnableLogger:               true,
		EnableWatcher:              false,
		DownloadImages:             false,
		MoviesDir:                  sql.NullString{String: helpers.DEFAULT_MOVIES_DIR, Valid: true},
		ShowsDir:                   sql.NullString{String: helpers.DEFAULT_SHOWS_DIR, Valid: true},
		MusicDir:                   sql.NullString{String: helpers.DEFAULT_MUSIC_DIR, Valid: true},
		StaticDir:                  helpers.DEFAULT_STATIC_DIR,
		LogsDir:                    helpers.DEFAULT_LOGS_DIR,
	}
	_, err := app.Queries.CreateSettings(context.Background(), params)
	if err != nil {
		t.Fatalf("Failed to create test settings: %v", err)
	}

	clearSettingsEnv(t)
	t.Setenv("TMDB_API_KEY", "override-key")
	t.Setenv("JELLYFIN_TOKEN", "override-jellyfin")
	t.Setenv("SPOTIFY_CLIENT_ID", "override-spotify-id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "override-spotify-secret")
	t.Setenv("HARDWARE_ACCELERATION_DEVICE", "nvidia")
	t.Setenv("ENABLE_LOGGER", "false")
	t.Setenv("ENABLE_WATCHER", "true")
	t.Setenv("DOWNLOAD_IMAGES", "true")

	err = app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	if app.Settings.TmdbKey.String != "override-key" {
		t.Errorf("Expected override tmdb key, got %q", app.Settings.TmdbKey.String)
	}
	if app.Settings.JellyfinToken.String != "override-jellyfin" {
		t.Errorf("Expected override jellyfin token, got %q", app.Settings.JellyfinToken.String)
	}
	if app.Settings.SpotifyClientID.String != "override-spotify-id" {
		t.Errorf("Expected override spotify client id, got %q", app.Settings.SpotifyClientID.String)
	}
	if app.Settings.SpotifyClientSecret.String != "override-spotify-secret" {
		t.Errorf("Expected override spotify client secret, got %q", app.Settings.SpotifyClientSecret.String)
	}
	if app.Settings.HardwareAccelerationDevice.String != "nvidia" {
		t.Errorf("Expected override hardware mode, got %q", app.Settings.HardwareAccelerationDevice.String)
	}
	if app.Settings.EnableLogger {
		t.Error("Expected ENABLE_LOGGER override to set false")
	}
	if !app.Settings.EnableWatcher {
		t.Error("Expected ENABLE_WATCHER override to set true")
	}
	if !app.Settings.DownloadImages {
		t.Error("Expected DOWNLOAD_IMAGES override to set true")
	}
	if app.Settings.MoviesDir.String != helpers.DEFAULT_MOVIES_DIR {
		t.Errorf("Expected MoviesDir to remain fixed at %q, got %q", helpers.DEFAULT_MOVIES_DIR, app.Settings.MoviesDir.String)
	}
	if app.Settings.ShowsDir.String != helpers.DEFAULT_SHOWS_DIR {
		t.Errorf("Expected ShowsDir to remain fixed at %q, got %q", helpers.DEFAULT_SHOWS_DIR, app.Settings.ShowsDir.String)
	}
	if app.Settings.MusicDir.String != helpers.DEFAULT_MUSIC_DIR {
		t.Errorf("Expected MusicDir to remain fixed at %q, got %q", helpers.DEFAULT_MUSIC_DIR, app.Settings.MusicDir.String)
	}
	if app.Settings.StaticDir != helpers.DEFAULT_STATIC_DIR {
		t.Errorf("Expected StaticDir to remain fixed at %q, got %q", helpers.DEFAULT_STATIC_DIR, app.Settings.StaticDir)
	}
	if app.Settings.LogsDir != helpers.DEFAULT_LOGS_DIR {
		t.Errorf("Expected LogsDir to remain fixed at %q, got %q", helpers.DEFAULT_LOGS_DIR, app.Settings.LogsDir)
	}
}

func TestInitSettings_Idempotent(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()

	// Call InitSettings twice
	err := app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("First InitSettings call failed: %v", err)
	}

	firstSettingsID := app.Settings.ID

	err = app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("Second InitSettings call failed: %v", err)
	}

	// Should load the same settings, not create a new one
	if app.Settings.ID != firstSettingsID {
		t.Errorf("Expected same settings ID %d, got %d", firstSettingsID, app.Settings.ID)
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

func TestInitSession_DefaultCookieSecureDisabled(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	t.Setenv(helpers.ENV_SESSION_COOKIE_SECURE, "")
	app.InitSession()

	if app.SessionManager.Cookie.Secure {
		t.Fatal("expected secure cookies to be disabled by default")
	}
}

func TestInitSession_UsesSessionCookieSecureEnv(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	t.Setenv(helpers.ENV_SESSION_COOKIE_SECURE, "true")
	app.InitSession()

	if !app.SessionManager.Cookie.Secure {
		t.Fatal("expected secure cookies to honor SESSION_COOKIE_SECURE=true")
	}
}

func TestInitDirs(t *testing.T) {
	tmpDir := t.TempDir()
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Settings = &database.Setting{
		StaticDir: filepath.Join(tmpDir, "static"),
		LogsDir:   filepath.Join(tmpDir, "logs"),
	}

	err := app.InitDirs()
	if err != nil {
		t.Fatalf("InitDirs failed: %v", err)
	}

	for _, dir := range []string{app.Settings.StaticDir, app.Settings.LogsDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}
}

func TestNullString(t *testing.T) {
	tests := []struct {
		input    string
		expected sql.NullString
	}{
		{"", sql.NullString{Valid: false}},
		{"value", sql.NullString{String: "value", Valid: true}},
	}

	for _, tt := range tests {
		result := helpers.NullString(tt.input)
		if result != tt.expected {
			t.Errorf("NullString(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
