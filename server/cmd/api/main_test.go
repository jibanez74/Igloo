package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	applogger "igloo/cmd/internal/logger"

	_ "github.com/mattn/go-sqlite3"
	cache "github.com/patrickmn/go-cache"
)

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

func clearRuntimeConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		helpers.ENV_DB_PATH,
		helpers.ENV_STATIC_DIR,
		helpers.ENV_LOGS_DIR,
		helpers.ENV_TRANSCODE_DIR,
		helpers.ENV_PORT,
		helpers.ENV_LOG_TO_STDOUT,
		helpers.ENV_SESSION_COOKIE_SECURE,
		"DEBUG",
	} {
		t.Setenv(key, "")
	}
}

func TestNewRuntimeConfig_DefaultPaths(t *testing.T) {
	clearRuntimeConfigEnv(t)

	cfg, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}

	if cfg.DBPath != helpers.DEFAULT_DB_PATH {
		t.Fatalf("expected derived DB path, got %q", cfg.DBPath)
	}
	if cfg.StaticDir != helpers.DEFAULT_STATIC_DIR {
		t.Fatalf("expected derived static dir, got %q", cfg.StaticDir)
	}
	if cfg.LogsDir != helpers.DEFAULT_LOGS_DIR {
		t.Fatalf("expected derived logs dir, got %q", cfg.LogsDir)
	}
	if cfg.TranscodeDir != helpers.DEFAULT_TRANSCODE_DIR {
		t.Fatalf("expected derived transcode dir, got %q", cfg.TranscodeDir)
	}
}

func TestNewRuntimeConfig_ExplicitPathsOverrideDefaults(t *testing.T) {
	clearRuntimeConfigEnv(t)

	dbPath := filepath.Join(t.TempDir(), "custom.db")
	staticDir := filepath.Join(t.TempDir(), "static")
	logsDir := filepath.Join(t.TempDir(), "logs")
	transcodeDir := filepath.Join(t.TempDir(), "transcode")

	t.Setenv(helpers.ENV_DB_PATH, dbPath)
	t.Setenv(helpers.ENV_STATIC_DIR, staticDir)
	t.Setenv(helpers.ENV_LOGS_DIR, logsDir)
	t.Setenv(helpers.ENV_TRANSCODE_DIR, transcodeDir)

	cfg, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}

	if cfg.DBPath != dbPath {
		t.Fatalf("expected DB path override %q, got %q", dbPath, cfg.DBPath)
	}
	if cfg.StaticDir != staticDir {
		t.Fatalf("expected static dir override %q, got %q", staticDir, cfg.StaticDir)
	}
	if cfg.LogsDir != logsDir {
		t.Fatalf("expected logs dir override %q, got %q", logsDir, cfg.LogsDir)
	}
	if cfg.TranscodeDir != transcodeDir {
		t.Fatalf("expected transcode dir override %q, got %q", transcodeDir, cfg.TranscodeDir)
	}
}

func TestNewRuntimeConfig_PortHonoredWithoutDebug(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv("DEBUG", "false")
	t.Setenv(helpers.ENV_PORT, "4242")

	cfg, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}

	if cfg.Port != 4242 {
		t.Fatalf("expected PORT to be honored without DEBUG, got %d", cfg.Port)
	}
}

func TestNewRuntimeConfig_RejectsInvalidPort(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv(helpers.ENV_PORT, "not-a-port")

	_, err := NewRuntimeConfig()
	if err == nil {
		t.Fatal("expected invalid PORT to return an error")
	}
}

func changeWorkingDirectory(t *testing.T, dir string) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func TestLoadRuntimeEnvFile_LoadsWorkingDirectoryEnvFile(t *testing.T) {
	envDir := t.TempDir()
	changeWorkingDirectory(t, envDir)

	if err := os.WriteFile(".env", []byte("IGLOO_TEST_ENV_FILE_VALUE=loaded\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	const key = "IGLOO_TEST_ENV_FILE_VALUE"
	old, hadOld := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if hadOld {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})

	envFile, loaded, err := LoadRuntimeEnvFile()
	if err != nil {
		t.Fatalf("LoadRuntimeEnvFile failed: %v", err)
	}

	if !loaded || envFile != ".env" {
		t.Fatalf("expected .env to be loaded, got file=%q loaded=%t", envFile, loaded)
	}
	if got := os.Getenv(key); got != "loaded" {
		t.Fatalf("expected env file value to load, got %q", got)
	}
}

func TestLoadRuntimeEnvFile_MissingEnvFileIsIgnored(t *testing.T) {
	changeWorkingDirectory(t, t.TempDir())

	envFile, loaded, err := LoadRuntimeEnvFile()
	if err != nil {
		t.Fatalf("LoadRuntimeEnvFile failed: %v", err)
	}
	if loaded || envFile != "" {
		t.Fatalf("expected missing .env to be ignored, got file=%q loaded=%t", envFile, loaded)
	}
}

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
}

func TestInitDB_DefaultPath(t *testing.T) {
	tmpDir := t.TempDir()
	changeWorkingDirectory(t, tmpDir)
	t.Setenv(helpers.ENV_DB_PATH, "")

	dbFile := filepath.Join(tmpDir, helpers.DEFAULT_DB_PATH)

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
		INSERT INTO settings (tmdb_key, static_dir)
		VALUES ('first-key', 'first-static')
	`)
	if err != nil {
		t.Fatalf("insert first settings row: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO settings (tmdb_key, static_dir)
		VALUES ('second-key', 'second-static')
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

	_, err = db.Exec(`
		INSERT INTO musicians (name, sort_name) 
		VALUES ('Test Artist', 'test artist')
	`)

	if err != nil {
		t.Fatalf("Failed to insert musician: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO albums (title, sort_title) 
		VALUES ('Test Album', 'test album')
	`)
	if err != nil {
		t.Fatalf("Failed to insert album: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO musician_albums (musician_id, album_id) 
		VALUES (1, 1)
	`)

	if err != nil {
		t.Fatalf("Failed to insert musician_album relationship: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO musician_albums (musician_id, album_id) 
		VALUES (999, 1)
	`)
	if err == nil {
		t.Error("Expected foreign key constraint violation, but insert succeeded")
	}
}

func setupTestApp(t *testing.T) *Application {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	dataDir := t.TempDir()
	app := &Application{
		DB: db,
		Config: RuntimeConfig{
			TranscodeDir: filepath.Join(dataDir, "transcode"),
			Port:         helpers.DEFAULT_APP_PORT,
		},
	}
	setupTestLogger(t, app)

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	app.Queries, err = database.Prepare(context.Background(), db)
	if err != nil {
		t.Fatalf("Failed to prepare queries: %v", err)
	}

	// Tests do not attach real FFmpeg processes to HLS cache entries.
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(100)
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
		"JELLYFIN_API_KEY",
		"SPOTIFY_CLIENT_ID",
		"SPOTIFY_CLIENT_SECRET",
		"HARDWARE_ACCELERATION_DEVICE",
		"ENABLE_WATCHER",
		"DOWNLOAD_IMAGES",
		"STATIC_DIR",
		"LOGS_DIR",
		"TRANSCODE_DIR",
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

	if app.Settings == nil {
		t.Fatal("Settings should not be nil after InitSettings")
	}

	if app.Settings.StaticDir != helpers.DEFAULT_STATIC_DIR {
		t.Errorf("Expected StaticDir %q, got %q", helpers.DEFAULT_STATIC_DIR, app.Settings.StaticDir)
	}
	if app.Settings.TranscodeDir != helpers.DEFAULT_TRANSCODE_DIR {
		t.Errorf("Expected TranscodeDir %q, got %q", helpers.DEFAULT_TRANSCODE_DIR, app.Settings.TranscodeDir)
	}

	if app.Settings.HardwareAccelerationDevice.String != "cpu" {
		t.Errorf("Expected HardwareAccelerationDevice 'cpu', got '%s'", app.Settings.HardwareAccelerationDevice.String)
	}
	if !app.Settings.HardwareAccelerationDevice.Valid {
		t.Error("Expected HardwareAccelerationDevice to be valid")
	}

	if app.Settings.EnableWatcher != false {
		t.Error("Expected EnableWatcher to be false by default")
	}
	if app.Settings.DownloadImages != false {
		t.Error("Expected DownloadImages to be false by default")
	}

	if app.Settings.TmdbKey.Valid {
		t.Error("Expected TmdbKey to be invalid when not set")
	}
	if app.Settings.JellyfinApiKey.Valid {
		t.Error("Expected JellyfinApiKey to be invalid when not set")
	}
	if app.Settings.MoviesDir.Valid {
		t.Errorf("Expected MoviesDir to be disabled by default, got %q", app.Settings.MoviesDir.String)
	}
	if app.Settings.ShowsDir.Valid {
		t.Errorf("Expected ShowsDir to be disabled by default, got %q", app.Settings.ShowsDir.String)
	}
	if app.Settings.MusicDir.Valid {
		t.Errorf("Expected MusicDir to be disabled by default, got %q", app.Settings.MusicDir.String)
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

	ctx := context.Background()
	err := app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	if app.Settings.TmdbKey.String != "test-tmdb-key" || !app.Settings.TmdbKey.Valid {
		t.Errorf("Expected TmdbKey 'test-tmdb-key' (valid), got '%s' (valid=%v)", app.Settings.TmdbKey.String, app.Settings.TmdbKey.Valid)
	}
	if app.Settings.JellyfinApiKey.String != "test-jellyfin-api-key" || !app.Settings.JellyfinApiKey.Valid {
		t.Errorf("Expected JellyfinApiKey 'test-jellyfin-api-key' (valid), got '%s' (valid=%v)", app.Settings.JellyfinApiKey.String, app.Settings.JellyfinApiKey.Valid)
	}
	if app.Settings.HardwareAccelerationDevice.String != "nvidia" || !app.Settings.HardwareAccelerationDevice.Valid {
		t.Errorf("Expected HardwareAccelerationDevice 'nvidia' (valid), got '%s' (valid=%v)", app.Settings.HardwareAccelerationDevice.String, app.Settings.HardwareAccelerationDevice.Valid)
	}
	if app.Settings.MoviesDir.String != "/host/movies" || !app.Settings.MoviesDir.Valid {
		t.Errorf("Expected MoviesDir %q (valid), got %q (valid=%v)", "/host/movies", app.Settings.MoviesDir.String, app.Settings.MoviesDir.Valid)
	}
	if app.Settings.ShowsDir.String != "/host/shows" || !app.Settings.ShowsDir.Valid {
		t.Errorf("Expected ShowsDir %q (valid), got %q (valid=%v)", "/host/shows", app.Settings.ShowsDir.String, app.Settings.ShowsDir.Valid)
	}
	if app.Settings.MusicDir.String != "/host/music" || !app.Settings.MusicDir.Valid {
		t.Errorf("Expected MusicDir %q (valid), got %q (valid=%v)", "/host/music", app.Settings.MusicDir.String, app.Settings.MusicDir.Valid)
	}

	if app.Settings.StaticDir != "/host/static" {
		t.Errorf("Expected StaticDir %q, got %q", "/host/static", app.Settings.StaticDir)
	}
	if app.Settings.TranscodeDir != "/host/transcode" {
		t.Errorf("Expected TranscodeDir %q, got %q", "/host/transcode", app.Settings.TranscodeDir)
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
		JellyfinApiKey:             sql.NullString{Valid: false},
		HardwareAccelerationDevice: sql.NullString{String: "nvidia", Valid: true},
		EnableWatcher:              false,
		DownloadImages:             false,
		MoviesDir:                  sql.NullString{Valid: false},
		ShowsDir:                   sql.NullString{Valid: false},
		MusicDir:                   sql.NullString{Valid: false},
		StaticDir:                  "existing-static",
		TranscodeDir:               "existing-transcode",
	}
	_, err := app.Queries.CreateSettings(context.Background(), params)
	if err != nil {
		t.Fatalf("Failed to create test settings: %v", err)
	}

	err = app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	if app.Settings.TmdbKey.String != "existing-key" {
		t.Errorf("Expected TmdbKey 'existing-key', got '%s'", app.Settings.TmdbKey.String)
	}
	if app.Settings.StaticDir != "existing-static" {
		t.Errorf("Expected StaticDir 'existing-static', got '%s'", app.Settings.StaticDir)
	}
	if app.Settings.TranscodeDir != "existing-transcode" {
		t.Errorf("Expected TranscodeDir 'existing-transcode', got '%s'", app.Settings.TranscodeDir)
	}
	if app.Settings.HardwareAccelerationDevice.String != "nvidia" {
		t.Errorf("Expected HardwareAccelerationDevice 'nvidia', got '%s'", app.Settings.HardwareAccelerationDevice.String)
	}
}

func TestInitSettings_ExistingSettingsIgnoreEnvOverrides(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

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
		StaticDir:                  helpers.DEFAULT_STATIC_DIR,
		TranscodeDir:               helpers.DEFAULT_TRANSCODE_DIR,
	}
	_, err := app.Queries.CreateSettings(context.Background(), params)
	if err != nil {
		t.Fatalf("Failed to create test settings: %v", err)
	}

	clearSettingsEnv(t)
	t.Setenv("TMDB_API_KEY", "override-key")
	t.Setenv("JELLYFIN_API_KEY", "override-jellyfin")
	t.Setenv("SPOTIFY_CLIENT_ID", "override-spotify-id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "override-spotify-secret")
	t.Setenv("HARDWARE_ACCELERATION_DEVICE", "nvidia")
	t.Setenv("ENABLE_WATCHER", "true")
	t.Setenv("DOWNLOAD_IMAGES", "true")
	t.Setenv("MOVIES_DIR", "/override/movies")
	t.Setenv("SHOWS_DIR", "/override/shows")
	t.Setenv("MUSIC_DIR", "/override/music")
	t.Setenv("STATIC_DIR", "/override/static")
	t.Setenv("TRANSCODE_DIR", "/override/transcode")

	err = app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("InitSettings failed: %v", err)
	}

	if app.Settings.TmdbKey.String != "existing-key" {
		t.Errorf("Expected existing tmdb key, got %q", app.Settings.TmdbKey.String)
	}
	if app.Settings.JellyfinApiKey.String != "existing-jellyfin" {
		t.Errorf("Expected existing jellyfin api key, got %q", app.Settings.JellyfinApiKey.String)
	}
	if app.Settings.SpotifyClientID.String != "existing-spotify-id" {
		t.Errorf("Expected existing spotify client id, got %q", app.Settings.SpotifyClientID.String)
	}
	if app.Settings.SpotifyClientSecret.String != "existing-spotify-secret" {
		t.Errorf("Expected existing spotify client secret, got %q", app.Settings.SpotifyClientSecret.String)
	}
	if app.Settings.HardwareAccelerationDevice.String != "cpu" {
		t.Errorf("Expected existing hardware mode, got %q", app.Settings.HardwareAccelerationDevice.String)
	}
	if app.Settings.EnableWatcher {
		t.Error("Expected existing EnableWatcher to remain false")
	}
	if app.Settings.DownloadImages {
		t.Error("Expected existing DownloadImages to remain false")
	}
	if app.Settings.MoviesDir.String != existingMoviesDir {
		t.Errorf("Expected MoviesDir to remain fixed at %q, got %q", existingMoviesDir, app.Settings.MoviesDir.String)
	}
	if !app.Settings.MoviesDir.Valid {
		t.Error("Expected MoviesDir.Valid to remain true")
	}
	if app.Settings.ShowsDir.String != existingShowsDir {
		t.Errorf("Expected ShowsDir to remain fixed at %q, got %q", existingShowsDir, app.Settings.ShowsDir.String)
	}
	if !app.Settings.ShowsDir.Valid {
		t.Error("Expected ShowsDir.Valid to remain true")
	}
	if app.Settings.MusicDir.String != existingMusicDir {
		t.Errorf("Expected MusicDir to remain fixed at %q, got %q", existingMusicDir, app.Settings.MusicDir.String)
	}
	if !app.Settings.MusicDir.Valid {
		t.Error("Expected MusicDir.Valid to remain true")
	}
	if app.Settings.StaticDir != helpers.DEFAULT_STATIC_DIR {
		t.Errorf("Expected StaticDir to remain fixed at %q, got %q", helpers.DEFAULT_STATIC_DIR, app.Settings.StaticDir)
	}
	if app.Settings.TranscodeDir != helpers.DEFAULT_TRANSCODE_DIR {
		t.Errorf("Expected TranscodeDir to remain fixed at %q, got %q", helpers.DEFAULT_TRANSCODE_DIR, app.Settings.TranscodeDir)
	}
}

func TestInitSettings_Idempotent(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()

	err := app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("First InitSettings call failed: %v", err)
	}

	firstSettingsID := app.Settings.ID

	err = app.InitSettings(ctx)
	if err != nil {
		t.Fatalf("Second InitSettings call failed: %v", err)
	}

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

	moviesDir := filepath.Join(tmpDir, "movies")
	if err := os.Mkdir(moviesDir, 0o755); err != nil {
		t.Fatalf("failed to create movies directory: %v", err)
	}

	missingShowsDir := filepath.Join(tmpDir, "shows")
	musicFile := filepath.Join(tmpDir, "music-file")
	if err := os.WriteFile(musicFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("failed to create music file: %v", err)
	}

	app.Settings = &database.Setting{
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
	}

	err := app.InitDirs()
	if err != nil {
		t.Fatalf("InitDirs failed: %v", err)
	}

	for _, dir := range []string{
		app.Settings.StaticDir,
		app.Settings.TranscodeDir,
		filepath.Join(app.Settings.StaticDir, "albums"),
		filepath.Join(app.Settings.StaticDir, "musicians"),
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

	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String != moviesDir {
		t.Errorf("expected existing movies directory to remain configured, got %q (valid=%v)", app.Settings.MoviesDir.String, app.Settings.MoviesDir.Valid)
	}
	if app.Settings.ShowsDir.Valid {
		t.Errorf("expected missing shows directory to be disabled, got %q", app.Settings.ShowsDir.String)
	}
	if app.Settings.MusicDir.Valid {
		t.Errorf("expected non-directory music path to be disabled, got %q", app.Settings.MusicDir.String)
	}
	if _, err := os.Stat(missingShowsDir); !os.IsNotExist(err) {
		t.Errorf("expected missing shows directory not to be created, stat err=%v", err)
	}
}
