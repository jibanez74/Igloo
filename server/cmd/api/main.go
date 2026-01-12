package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	applogger "igloo/cmd/internal/logger"
	"igloo/cmd/internal/spotify"
	"igloo/cmd/internal/tmdb"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for database/sql
)

type Application struct {
	DB             *sql.DB
	Queries        *database.Queries
	Settings       *database.Setting
	Logger         *slog.Logger
	LoggerCloser   func() error
	Ffprobe        ffprobe.FfprobeInterface
	Spotify        spotify.SpotifyInterface
	Tmdb           tmdb.TmdbInterface
	SessionManager *scs.SessionManager
	Wait           *sync.WaitGroup
	Router         *chi.Mux
	Server         *http.Server
}

// SQL contains the database schema, embedded at compile time.
// This ensures the schema is always bundled with the binary and

//go:embed schema.sql
var SQL string

func main() {
	log.Println("igloo server starting up...")

	// Load environment variables from .env file.
	// This allows local development without setting system env vars.
	// In production, env vars can come from Docker, systemd, or other sources.
	// Not sure if I'll keep this package for env vars, but for now will use it
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	app, err := InitApp()
	if err != nil {
		log.Fatal(err)
	}

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 8080
	}

	app.Server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: app.Router,
	}

	go app.ListenForShutdown()

	log.Printf("server listening on port %d", port)

	err = app.Server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// InitApp creates and initializes all application components in the correct order.
// The initialization sequence is critical - each step depends on the previous:
func InitApp() (*Application, error) {
	app := Application{
		Wait: &sync.WaitGroup{},
	}

	// Create a background context for database operations during startup.
	ctx := context.Background()

	// Initialize the logger first so all other init functions can log errors.
	// Uses environment variables directly since settings aren't loaded yet.
	// In debug mode logs to stdout, otherwise logs to file with rotation.
	err := app.InitLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	// Initialize database connection and create the database file if it doesn't exist.
	err = app.InitDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	// Create database tables if they don't exist.
	// Uses the embedded schema.sql with CREATE TABLE IF NOT EXISTS.
	err = app.InitTables()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database tables: %v", err)
	}

	// Get the prepared queries from the database package.
	// The base queries are stored in sqlc/queries and sqlc generates the prepared queries.
	app.Queries, err = database.Prepare(ctx, app.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare database queries: %v", err)
	}

	// Load or create application settings.
	// Reads existing settings from DB, or creates defaults from env vars.
	err = app.InitSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize settings: %v", err)
	}

	// Create required directories (static, logs) and optional media directories.
	// Must run after InitSettings since directory paths come from settings.
	err = app.InitDirs()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize directories: %v", err)
	}

	// Ensure a default admin user exists.
	// Creates admin@sample.com with password "AdminPassword" if no admin found.
	err = app.InitDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize default user: %v", err)
	}

	// Initialize session manager for authentication.
	// Uses SQLite as the session store, sharing the same database connection.
	app.InitSession()

	// Initialize ffprobe for media metadata extraction.
	// Extracts the platform-specific binary from embedded data to a temp directory.
	ffprobeApp, err := ffprobe.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffprobe: %v", err)
	}
	app.Ffprobe = ffprobeApp

	// Initialize Spotify client if credentials are configured.
	// This is optional - the app works without Spotify integration.
	if app.Settings.SpotifyClientID.Valid && app.Settings.SpotifyClientSecret.Valid {
		s, err := spotify.New(app.Settings.SpotifyClientID.String, app.Settings.SpotifyClientSecret.String)
		if err != nil {
			app.Logger.Warn("failed to initialize spotify client", "error", err)
		} else {
			app.Spotify = s
			app.Logger.Info("spotify client initialized successfully")
		}
	}

	// initialize tmdb client if tmdb key is configured
	if app.Settings.TmdbKey.Valid {
		tmdb, err := tmdb.New(app.Settings.TmdbKey.String)
		if err != nil {
			app.Logger.Warn("failed to initialize tmdb client", "error", err)
		} else {
			app.Tmdb = tmdb
		}
	}

	// Start music library scanner in background if music directory is configured.
	if app.Settings.MusicDir.Valid && app.Settings.MusicDir.String != "" {
		go app.ScanMusicLibrary()
	}

	app.InitRouter()

	return &app, nil
}

func (app *Application) InitDB() error {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "igloo.db"
	}

	_, err := os.Stat(dbPath)
	if err == nil {
		app.Logger.Info("opening existing database", "path", dbPath)
	} else if os.IsNotExist(err) {
		app.Logger.Info("creating new database", "path", dbPath)
	} else {
		return err
	}

	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on")
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return err
	}

	app.DB = db

	return nil
}

// InitTables executes the embedded schema.sql to create all database tables.
// Uses CREATE TABLE IF NOT EXISTS so it's safe to run on every startup.
// This ensures the database schema is always up to date with the application.
func (app *Application) InitTables() error {
	_, err := app.DB.Exec(SQL)
	if err != nil {
		return err
	}

	app.Logger.Info("database tables initialized successfully")

	return nil
}

// InitSettings loads application settings from the database.
// If no settings exist (first run), creates a new settings record
// populated from environment variables with sensible defaults.
func (app *Application) InitSettings(ctx context.Context) error {
	settings, err := app.Queries.GetSettings(ctx)
	if err == nil {
		// Settings exist - use them.
		app.Logger.Info("loaded existing settings from database")
		app.Settings = &settings
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	app.Logger.Info("no settings found, creating default settings...")

	downloadImages, _ := strconv.ParseBool(os.Getenv("DOWNLOAD_IMAGES"))
	enableLogger, _ := strconv.ParseBool(os.Getenv("ENABLE_LOGGER"))
	enableWatcher, _ := strconv.ParseBool(os.Getenv("ENABLE_WATCHER"))

	logsDir := os.Getenv("LOGS_DIR")
	if logsDir == "" {
		logsDir = "logs"
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}

	// Hardware acceleration defaults to CPU (no acceleration).
	hardwareAccelerationDevice := os.Getenv("HARDWARE_ACCELERATION_DEVICE")
	if hardwareAccelerationDevice == "" {
		hardwareAccelerationDevice = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	}

	// Build the settings record from environment variables.
	// NullString handles empty strings by setting Valid=false.
	params := database.CreateSettingsParams{
		TmdbKey:                    helpers.NullString(os.Getenv("TMDB_API_KEY")),
		JellyfinToken:              helpers.NullString(os.Getenv("JELLYFIN_TOKEN")),
		SpotifyClientID:            helpers.NullString(os.Getenv("SPOTIFY_CLIENT_ID")),
		SpotifyClientSecret:        helpers.NullString(os.Getenv("SPOTIFY_CLIENT_SECRET")),
		HardwareAccelerationDevice: helpers.NullString(hardwareAccelerationDevice),
		EnableLogger:               enableLogger,
		EnableWatcher:              enableWatcher,
		DownloadImages:             downloadImages,
		MoviesDir:                  helpers.NullString(os.Getenv("MOVIES_DIR")),
		ShowsDir:                   helpers.NullString(os.Getenv("SHOWS_DIR")),
		MusicDir:                   helpers.NullString(os.Getenv("MUSIC_DIR")),
		StaticDir:                  staticDir,
		LogsDir:                    logsDir,
	}

	settings, err = app.Queries.CreateSettings(ctx, params)
	if err != nil {
		return err
	}

	app.Logger.Info("default settings created successfully")

	app.Settings = &settings

	return nil
}

// InitDirs ensures all required directories exist, creating them if necessary.
// Required directories (static, logs) are always created.
// Optional media directories (movies, shows, music) are only created if configured.
func (app *Application) InitDirs() error {
	// Create required directories - these are needed for the app to function.
	created, err := helpers.GetOrCreateDir(app.Settings.StaticDir)
	if err != nil {
		return fmt.Errorf("failed to initialize static directory: %w", err)
	}

	if created {
		app.Logger.Info("created static directory", "path", app.Settings.StaticDir)
	}

	created, err = helpers.GetOrCreateDir(app.Settings.LogsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize logs directory: %w", err)
	}

	if created {
		app.Logger.Info("created logs directory", "path", app.Settings.LogsDir)
	}

	// Create optional media directories only if they are configured.
	if app.Settings.MoviesDir.Valid {
		created, err = helpers.GetOrCreateDir(app.Settings.MoviesDir.String)
		if err != nil {
			app.Logger.Error("failed to initialize movies directory", "error", err)
		}

		if created {
			app.Logger.Info("created movies directory", "path", app.Settings.MoviesDir.String)
		}
	}

	if app.Settings.ShowsDir.Valid {
		created, err = helpers.GetOrCreateDir(app.Settings.ShowsDir.String)
		if err != nil {
			app.Logger.Error("failed to initialize shows directory", "error", err)
		}

		if created {
			app.Logger.Info("created shows directory", "path", app.Settings.ShowsDir.String)
		}
	}

	if app.Settings.MusicDir.Valid {
		created, err = helpers.GetOrCreateDir(app.Settings.MusicDir.String)
		if err != nil {
			app.Logger.Error("failed to initialize music directory", "error", err)
		}

		if created {
			app.Logger.Info("created music directory", "path", app.Settings.MusicDir.String)
		}
	}

	app.Logger.Info("directories initialized successfully")

	return nil
}

// InitLogger initializes the application logger.
// In debug mode (DEBUG=true), logs are written to stdout with text format.
// In production mode, logs are written to a file with JSON format and rotation.
// Uses LOGS_DIR environment variable (defaults to "logs").
func (app *Application) InitLogger() error {
	debug := os.Getenv("DEBUG") == "true"

	logsDir := os.Getenv("LOGS_DIR")
	if logsDir == "" {
		logsDir = "logs"
	}

	// Create logs directory if not in debug mode (file logging requires it).
	if !debug {
		_, err := helpers.GetOrCreateDir(logsDir)
		if err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}
	}

	logger, closer, err := applogger.New(&applogger.LoggerConfig{
		Debug:   debug,
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

// InitDefaultUser ensures an admin user exists in the database.
// On first run, creates a default admin account that can be used
// to access the application and create additional users.
// Credentials: admin@sample.com / AdminPassword
// The password should be changed after first login.
func (app *Application) InitDefaultUser(ctx context.Context) error {
	_, err := app.Queries.GetAdminUser(ctx)
	if err == nil {
		// An admin exists - nothing to do.
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	app.Logger.Info("no admin user found, creating default admin user...")

	hashedPassword, err := helpers.HashPassword("AdminPassword")
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	params := database.CreateUserParams{
		Name:     "Admin",
		Email:    "admin@sample.com",
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

// InitSession initializes the session manager with SQLite as the session store.
// Sessions are used for authentication and maintaining user state across requests.
// The sessions table is created by InitTables via schema.sql.
func (app *Application) InitSession() {
	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(app.DB)
	sessionManager.Lifetime = 30 * 24 * time.Hour
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = os.Getenv("DEBUG") != "true"

	app.SessionManager = sessionManager

	app.Logger.Info("session manager initialized successfully")
}

func (app *Application) InitRouter() {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(app.LoadAndSaveSession)

	router.Route("/api", func(r chi.Router) {
		r.Get("/health", app.HealthCheck)

		r.Route("/auth", func(r chi.Router) {
			r.Get("/user", app.GetCurrentAuthUser)
			r.Post("/login", app.AuthenticateUser)
			r.Delete("/logout", app.DestroySession)
		})

		r.Route("/tmdb", func(r chi.Router) {
			r.Get("/movies/in-theaters", app.GetMoviesInTheaters)
			r.Get("/movies/{id}", app.GetMovieByTmdbID)
		})

		r.Route("/music", func(r chi.Router) {
			r.Get("/stats", app.GetMusicStats)

			r.Route("/albums", func(r chi.Router) {
				r.Get("/details/{id}", app.GetAlbumDetails)
				r.Get("/latest", app.GetLatestAlbums)
			})

		r.Route("/tracks", func(r chi.Router) {
			r.Get("/", app.GetTracksAlphabetical)
			r.Get("/shuffle", app.GetShuffleTracks)
			r.Get("/details/{id}", app.GetTrackByID)
			r.Get("/{id}/stream", app.StreamTrack)
		})
		})
	})

	app.Router = router
}

// ListenForShutdown handles graceful shutdown when SIGINT or SIGTERM is received.
// This is typically triggered by Ctrl+C, `kill`, or container orchestrators.
func (app *Application) ListenForShutdown() {
	// Create a channel to receive OS signals.
	quit := make(chan os.Signal, 1)

	// Register for interrupt (Ctrl+C) and terminate signals.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received.
	<-quit

	// Stop receiving further signals.
	signal.Stop(quit)

	app.Logger.Info("shutting down server...")

	// Create a context with timeout for graceful shutdown.
	// Gives in-flight requests 10 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Gracefully shutdown the HTTP server.
	// This stops accepting new requests and waits for in-flight requests to complete.
	if app.Server != nil {
		err := app.Server.Shutdown(ctx)
		if err != nil {
			app.Logger.Error("failed to shutdown server", "error", err)
		}
	}

	app.Logger.Info("running clean up tasks...")

	// Wait for any in-flight background tasks to complete.
	// These may still need database and logger access.
	app.Wait.Wait()

	// Clean up ffprobe temp directory and extracted binary.
	err := ffprobe.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffprobe", "error", err)
	}

	// Close database connection to ensure all writes are flushed.
	// Done after HTTP and background tasks are complete.
	if app.DB != nil {
		err = app.DB.Close()
		if err != nil {
			app.Logger.Error("failed to close database", "error", err)
		}
	}

	// Close the logger last to flush any remaining buffered logs.
	// This ensures we can log errors from all previous cleanup steps.
	// Use standard log here since app.Logger is being closed.
	if app.LoggerCloser != nil {
		err = app.LoggerCloser()
		if err != nil {
			log.Printf("failed to close logger: %v", err)
		}
	}

	os.Exit(0)
}
