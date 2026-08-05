package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	applogger "igloo/cmd/internal/logger"
	"igloo/sqlc"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for database/sql
)

func (app *Application) InitDB() error {
	dbPath := app.Config.effectiveDBPath()

	dir := filepath.Dir(dbPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create database directory %s for %s: %w", dir, dbPath, err)
	}

	_, err = os.Stat(dbPath)
	if err == nil {
		app.Logger.Info("opening existing database", "path", dbPath)
	} else if os.IsNotExist(err) {
		app.Logger.Info("creating new database", "path", dbPath)
	} else {

		return fmt.Errorf("failed to stat database path %s: %w", dbPath, err)
	}

	err = ensureDatabasePathWritable(dbPath, err == nil)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	err = db.Ping()
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to database %s: %w", dbPath, err)
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to enable WAL journal mode for database %s: %w", dbPath, err)
	}

	// These are connection-scoped, which is safe because the pool is pinned to a
	// single connection above.
	//
	// synchronous=NORMAL is the standard pairing with WAL: a commit no longer waits
	// on an fsync, and the scanners commit once per movie or track. WAL still
	// guarantees the database survives a process crash; only an OS-level crash can
	// lose the most recent commits.
	//
	// temp_store=MEMORY keeps the sorters and materialized subqueries the library
	// listings build off the disk.
	pragmas := []string{
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA temp_store=MEMORY;",
		"PRAGMA cache_size=-16000;",
	}

	for _, pragma := range pragmas {
		_, err = db.Exec(pragma)
		if err != nil {
			db.Close()
			return fmt.Errorf("failed to apply %q to database %s: %w", pragma, dbPath, err)
		}
	}

	app.DB = db

	return nil
}

func ensureDatabasePathWritable(dbPath string, databaseExists bool) error {
	dir := filepath.Dir(dbPath)

	probe, err := os.CreateTemp(dir, ".igloo-db-write-test-*")
	if err != nil {
		return fmt.Errorf("database directory is not writable for %s (%s): %w", dbPath, dir, err)
	}

	probePath := probe.Name()

	err = probe.Close()
	if err != nil {
		os.Remove(probePath)
		return fmt.Errorf("failed to close database directory write check for %s (%s): %w", dbPath, dir, err)
	}

	err = os.Remove(probePath)
	if err != nil {
		return fmt.Errorf("failed to remove database directory write check file for %s (%s): %w", dbPath, dir, err)
	}

	if !databaseExists {
		return nil
	}

	f, err := os.OpenFile(dbPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("database file is not writable at %s: %w", dbPath, err)
	}

	closeErr := f.Close()
	if closeErr != nil {
		return fmt.Errorf("failed to close database write check for %s: %w", dbPath, closeErr)
	}

	return nil
}

func (app *Application) InitTables() error {
	_, err := app.DB.Exec(sqlc.Schema)
	if err != nil {
		return err
	}

	app.Logger.Info("database tables initialized successfully")

	return nil
}

// RefreshQueryPlannerStats keeps sqlite_stat1 current so the planner picks the
// library listing indexes on their real selectivity instead of on built-in
// guesses. PRAGMA optimize only analyzes the tables that have changed enough to
// need it, and analysis_limit bounds how much of each index it samples, so this
// stays cheap even on a large library.
func (app *Application) RefreshQueryPlannerStats() error {
	_, err := app.DB.Exec("PRAGMA analysis_limit=400; PRAGMA optimize;")
	if err != nil {
		return fmt.Errorf("failed to refresh query planner statistics: %w", err)
	}

	return nil
}

func (app *Application) InitSettings(ctx context.Context) error {
	settings, err := app.Queries.GetSettings(ctx)
	if err == nil {
		app.Logger.Info("loaded existing settings from database")
		app.Settings = &settings
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	app.Logger.Info("no settings found, creating default settings...")

	params := database.CreateSettingsParams{
		TmdbKey:                    helpers.NullString(app.Config.TmdbAPIKey),
		JellyfinApiKey:             helpers.NullString(app.Config.JellyfinAPIKey),
		SpotifyClientID:            helpers.NullString(app.Config.SpotifyClientID),
		SpotifyClientSecret:        helpers.NullString(app.Config.SpotifyClientSecret),
		HardwareAccelerationDevice: helpers.NullString(app.Config.HardwareAccelerationDevice),
		EnableWatcher:              app.Config.EnableWatcher,
		DownloadImages:             app.Config.DownloadImages,
		MoviesDir:                  helpers.NullString(app.Config.MoviesDir),
		ShowsDir:                   helpers.NullString(app.Config.ShowsDir),
		MusicDir:                   helpers.NullString(app.Config.MusicDir),
		StaticDir:                  app.Config.effectiveStaticDir(),
		TranscodeDir:               app.Config.effectiveTranscodeDir(),
	}

	settings, err = app.Queries.CreateSettings(ctx, params)
	if err != nil {
		return err
	}

	app.Logger.Info("default settings created successfully")

	app.Settings = &settings

	return nil
}

func (app *Application) InitDirs() error {
	created, err := helpers.GetOrCreateDir(app.Settings.StaticDir)
	if err != nil {
		return fmt.Errorf("failed to initialize static directory: %w", err)
	}

	if created {
		app.Logger.Info("created static directory", "path", app.Settings.StaticDir)
	}

	// Scanner-downloaded artwork is stored beneath static/.
	_, err = helpers.GetOrCreateDir(filepath.Join(app.Settings.StaticDir, "albums"))
	if err != nil {
		return fmt.Errorf("failed to initialize static/albums: %w", err)
	}

	_, err = helpers.GetOrCreateDir(filepath.Join(app.Settings.StaticDir, "musicians"))
	if err != nil {
		return fmt.Errorf("failed to initialize static/musicians: %w", err)
	}

	transcodeDir := app.Settings.TranscodeDir
	if transcodeDir != "" {
		created, err = helpers.GetOrCreateDir(transcodeDir)
		if err != nil {
			return fmt.Errorf("failed to initialize transcode directory: %w", err)
		}

		if created {
			app.Logger.Info("created transcode directory", "path", transcodeDir)
		}
	}

	app.validateMediaDir("movies", &app.Settings.MoviesDir)
	app.validateMediaDir("shows", &app.Settings.ShowsDir)
	app.validateMediaDir("music", &app.Settings.MusicDir)

	app.Logger.Info("directories initialized successfully")

	return nil
}

func (app *Application) validateMediaDir(mediaType string, dir *sql.NullString) {
	if dir == nil || !dir.Valid || strings.TrimSpace(dir.String) == "" {
		if dir != nil {
			*dir = sql.NullString{}
		}

		return
	}

	dir.String = strings.TrimSpace(dir.String)
	validationErr := validateExistingDir(dir.String)
	if validationErr != nil {
		app.Logger.Warn("disabling inaccessible media directory", "type", mediaType, "path", dir.String, "error", validationErr)
		*dir = sql.NullString{}
	}
}

func validateExistingDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open directory: %w", err)
	}

	if err = dir.Close(); err != nil {
		return fmt.Errorf("failed to close directory: %w", err)
	}

	return nil
}

func (app *Application) InitLogger() error {
	debug := app.Config.Debug
	logToStdout := app.Config.LogToStdout

	logsDir := app.Config.effectiveLogsDir()

	if !logToStdout {
		_, err := helpers.GetOrCreateDir(logsDir)
		if err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}
	}

	logger, closer, err := applogger.New(&applogger.LoggerConfig{
		Debug:   debug,
		Stdout:  logToStdout,
		LogDir:  logsDir,
		LogFile: "igloo.log",
	})

	if err != nil {
		return err
	}

	app.Logger = logger
	app.LoggerCloser = closer

	app.Logger.Info("logger initialized successfully")

	return nil
}

func (app *Application) InitDefaultUser(ctx context.Context) error {
	_, err := app.Queries.GetAdminUser(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	app.Logger.Info("no admin user found, creating default admin user...")

	password := app.Config.DefaultAdminPassword
	if password == "" {
		return fmt.Errorf("%s must be set before creating the initial administrator", envDefaultAdminPassword)
	}

	hashedPassword, err := helpers.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	params := database.CreateUserParams{
		Name:     app.Config.DefaultAdminName,
		Email:    app.Config.DefaultAdminEmail,
		Password: hashedPassword,
		IsAdmin:  true,
		Avatar:   sql.NullString{Valid: false},
	}

	_, err = app.Queries.CreateUser(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create default admin user: %v", err)
	}

	app.Logger.Info("default admin user created successfully")

	return nil
}

func (app *Application) InitSession() {
	sessionManager := scs.New()
	sessionManager.Store = newCachedSessionStore(sqlite3store.New(app.DB))
	sessionManager.Lifetime = 30 * 24 * time.Hour
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = app.Config.SessionCookieSecure

	app.SessionManager = sessionManager

	app.Logger.Info("session manager initialized successfully", "cookie_secure", sessionManager.Cookie.Secure)
}

func cleanupStaleHLSTempDirs(logger applogger.LoggerInterface, transcodeDir string) {
	if strings.TrimSpace(transcodeDir) == "" {
		return
	}

	pattern := filepath.Join(transcodeDir, "igloo-hls-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.Warn("failed to glob stale HLS temp dirs", "error", err)
		return
	}
	for _, dir := range matches {
		err = os.RemoveAll(dir)
		if err != nil {
			logger.Warn("failed to remove stale HLS temp dir", "path", dir, "error", err)
		}
	}
	if len(matches) > 0 {
		logger.Info("cleaned up stale HLS temp dirs from previous run", "count", len(matches))
	}
}
