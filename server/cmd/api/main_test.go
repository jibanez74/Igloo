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
}

func TestInitDB_DefaultPath(t *testing.T) {
	// Ensure DB_PATH is not set
	os.Unsetenv("DB_PATH")

	// Change to temp directory so default igloo.db is created there
	originalDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	app := &Application{}
	setupTestLogger(t, app)

	err := app.InitDB()
	if err != nil {
		t.Fatalf("InitDB with default path failed: %v", err)
	}
	defer app.DB.Close()

	// Verify the default database file was created
	_, err = os.Stat(filepath.Join(tmpDir, "igloo.db"))
	if os.IsNotExist(err) {
		t.Error("Default database file 'igloo.db' was not created")
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

	return app
}

func TestInitSettings_CreatesDefaultSettings(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()

	// Clear any env vars that might affect the test.
	// These match the env vars read by InitSettings in main.go.
	envVars := []string{
		"TMDB_API_KEY", "JELLYFIN_TOKEN",
		"SPOTIFY_CLIENT_ID", "SPOTIFY_CLIENT_SECRET",
		"HARDWARE_ACCELERATION_DEVICE",
		"ENABLE_LOGGER", "ENABLE_WATCHER", "DOWNLOAD_IMAGES",
		"MOVIES_DIR", "SHOWS_DIR", "MUSIC_DIR",
		"STATIC_DIR", "LOGS_DIR",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	err := app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	// Verify settings were created and stored
	if app.Settings == nil {
		t.Fatal("Settings should not be nil after InitSettings")
	}

	// Verify default values for required string fields
	if app.Settings.StaticDir != "static" {
		t.Errorf("Expected StaticDir 'static', got '%s'", app.Settings.StaticDir)
	}
	if app.Settings.LogsDir != "logs" {
		t.Errorf("Expected LogsDir 'logs', got '%s'", app.Settings.LogsDir)
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
	if app.Settings.SpotifyClientID.Valid {
		t.Error("Expected SpotifyClientID to be invalid when not set")
	}
	if app.Settings.SpotifyClientSecret.Valid {
		t.Error("Expected SpotifyClientSecret to be invalid when not set")
	}
	if app.Settings.MoviesDir.Valid {
		t.Error("Expected MoviesDir to be invalid when not set")
	}
	if app.Settings.ShowsDir.Valid {
		t.Error("Expected ShowsDir to be invalid when not set")
	}
	if app.Settings.MusicDir.Valid {
		t.Error("Expected MusicDir to be invalid when not set")
	}
}

func TestInitSettings_UsesEnvVars(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	// Set all environment variables that InitSettings reads
	os.Setenv("TMDB_API_KEY", "test-tmdb-key")
	os.Setenv("JELLYFIN_TOKEN", "test-jellyfin-token")
	os.Setenv("SPOTIFY_CLIENT_ID", "test-spotify-id")
	os.Setenv("SPOTIFY_CLIENT_SECRET", "test-spotify-secret")
	os.Setenv("HARDWARE_ACCELERATION_DEVICE", "nvidia")
	os.Setenv("ENABLE_LOGGER", "true")
	os.Setenv("ENABLE_WATCHER", "true")
	os.Setenv("DOWNLOAD_IMAGES", "true")
	os.Setenv("MOVIES_DIR", "/movies")
	os.Setenv("SHOWS_DIR", "/shows")
	os.Setenv("MUSIC_DIR", "/music")
	os.Setenv("STATIC_DIR", "custom-static")
	os.Setenv("LOGS_DIR", "custom-logs")

	defer func() {
		os.Unsetenv("TMDB_API_KEY")
		os.Unsetenv("JELLYFIN_TOKEN")
		os.Unsetenv("SPOTIFY_CLIENT_ID")
		os.Unsetenv("SPOTIFY_CLIENT_SECRET")
		os.Unsetenv("HARDWARE_ACCELERATION_DEVICE")
		os.Unsetenv("ENABLE_LOGGER")
		os.Unsetenv("ENABLE_WATCHER")
		os.Unsetenv("DOWNLOAD_IMAGES")
		os.Unsetenv("MOVIES_DIR")
		os.Unsetenv("SHOWS_DIR")
		os.Unsetenv("MUSIC_DIR")
		os.Unsetenv("STATIC_DIR")
		os.Unsetenv("LOGS_DIR")
	}()

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
	if app.Settings.SpotifyClientID.String != "test-spotify-id" || !app.Settings.SpotifyClientID.Valid {
		t.Errorf("Expected SpotifyClientID 'test-spotify-id' (valid), got '%s' (valid=%v)", app.Settings.SpotifyClientID.String, app.Settings.SpotifyClientID.Valid)
	}
	if app.Settings.SpotifyClientSecret.String != "test-spotify-secret" || !app.Settings.SpotifyClientSecret.Valid {
		t.Errorf("Expected SpotifyClientSecret 'test-spotify-secret' (valid), got '%s' (valid=%v)", app.Settings.SpotifyClientSecret.String, app.Settings.SpotifyClientSecret.Valid)
	}
	if app.Settings.HardwareAccelerationDevice.String != "nvidia" || !app.Settings.HardwareAccelerationDevice.Valid {
		t.Errorf("Expected HardwareAccelerationDevice 'nvidia' (valid), got '%s' (valid=%v)", app.Settings.HardwareAccelerationDevice.String, app.Settings.HardwareAccelerationDevice.Valid)
	}
	if app.Settings.MoviesDir.String != "/movies" || !app.Settings.MoviesDir.Valid {
		t.Errorf("Expected MoviesDir '/movies' (valid), got '%s' (valid=%v)", app.Settings.MoviesDir.String, app.Settings.MoviesDir.Valid)
	}
	if app.Settings.ShowsDir.String != "/shows" || !app.Settings.ShowsDir.Valid {
		t.Errorf("Expected ShowsDir '/shows' (valid), got '%s' (valid=%v)", app.Settings.ShowsDir.String, app.Settings.ShowsDir.Valid)
	}
	if app.Settings.MusicDir.String != "/music" || !app.Settings.MusicDir.Valid {
		t.Errorf("Expected MusicDir '/music' (valid), got '%s' (valid=%v)", app.Settings.MusicDir.String, app.Settings.MusicDir.Valid)
	}

	// Verify required string fields from env vars
	if app.Settings.StaticDir != "custom-static" {
		t.Errorf("Expected StaticDir 'custom-static', got '%s'", app.Settings.StaticDir)
	}
	if app.Settings.LogsDir != "custom-logs" {
		t.Errorf("Expected LogsDir 'custom-logs', got '%s'", app.Settings.LogsDir)
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

	// Create settings directly in the database
	params := database.CreateSettingsParams{
		TmdbKey:                    sql.NullString{String: "existing-key", Valid: true},
		StaticDir:                  "existing-static",
		LogsDir:                    "existing-logs",
		HardwareAccelerationDevice: sql.NullString{String: "nvidia", Valid: true},
		EnableLogger:               true,
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
