package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
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
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type Application struct {
	DB               *sql.DB
	Queries          *database.Queries
	Settings         *database.Setting
	Logger           applogger.LoggerInterface
	LoggerCloser     func() error
	Ffprobe          ffprobe.FfprobeInterface
	FFmpeg           ffmpeg.FFmpegInterface
	Spotify          spotify.SpotifyInterface
	Tmdb             tmdb.TmdbInterface
	SessionManager   *scs.SessionManager
	Wait             *sync.WaitGroup
	Router           *chi.Mux
	Server           *http.Server
	ScannerDBMu      sync.Mutex
	HLSSessionCache  *cache.Cache
	HLSSessionGroup  singleflight.Group
	RemuxSafetyCache *cache.Cache
	SubtitleVTTCache *cache.Cache
	RoomHLSTombstone *cache.Cache
	RoomHLSMu        sync.Mutex
	WatchRoomHub     *WatchRoomHub
}

// SQL is the embedded startup schema applied by InitTables.
//
//go:embed schema.sql
var SQL string

// FrontendFS contains the embedded frontend bundle. A minimal placeholder is committed
// so the app builds without running the frontend build. When VITE_DEV_SERVER is set,
// ServeFrontend redirects to the Vite dev server instead of serving from here.
// all:webdist is required because Vite can emit chunk names starting with "_" and
// the default embed behavior skips files beginning with "_" or ".".
//
//go:embed all:webdist
var FrontendFS embed.FS

func main() {
	log.Println("igloo server starting up...")

	// Load .env for local development. Missing files are silently ignored;
	// only real read or parse failures are logged.
	err := godotenv.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("warning: failed to load .env: %v", err)
	}
	if err != nil {
		err = godotenv.Load("../.env")
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("warning: failed to load ../.env: %v", err)
		}
	}

	app, err := InitApp()
	if err != nil {
		log.Fatal(err)
	}

	port := helpers.DEFAULT_APP_PORT
	debug := os.Getenv("DEBUG") == "true"
	if debug {
		portStr := os.Getenv(helpers.ENV_PORT)
		if portStr != "" {
			p, err := strconv.Atoi(portStr)
			if err != nil {
				log.Fatalf("invalid PORT value %q: %v", portStr, err)
			}
			port = p
		}
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

// InitApp wires startup dependencies in dependency order.
func InitApp() (*Application, error) {
	app := Application{
		Wait: &sync.WaitGroup{},
	}

	ctx := context.Background()

	// Logger comes first so later startup failures are captured consistently.
	err := app.InitLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	// Remove any leftover HLS temp directories from a previous run that did not
	// shut down cleanly (crash, SIGKILL from systemd, power loss, etc.).
	cleanupStaleHLSTempDirs(app.Logger)

	err = app.InitDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	err = app.InitTables()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database tables: %v", err)
	}

	err = app.ensureSearchIndexesCurrent()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize search indexes: %v", err)
	}

	app.Queries, err = database.Prepare(ctx, app.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare database queries: %v", err)
	}

	err = app.InitSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize settings: %v", err)
	}

	app.WatchRoomHub = NewWatchRoomHub()

	// Directory paths come from settings, so this must run after InitSettings.
	err = app.InitDirs()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize directories: %v", err)
	}

	err = app.InitDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize default user: %v", err)
	}

	app.InitSession()

	ffprobeApp, err := ffprobe.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffprobe: %v", err)
	}
	app.Ffprobe = ffprobeApp

	ffmpegApp, err := ffmpeg.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffmpeg: %v", err)
	}
	app.FFmpeg = ffmpegApp

	// Eviction callback removes generated files when an HLS session ages out.
	hlsCache := cache.New(helpers.HLS_SESSION_TTL, helpers.HLS_SESSION_CACHE_SWEEP)
	hlsCache.OnEvicted(func(key string, val interface{}) {
		session, ok := val.(*HLSSession)
		if ok {
			cleanupHLSSession(session)
		}
	})
	app.HLSSessionCache = hlsCache
	app.RemuxSafetyCache = cache.New(
		helpers.HLS_REMUX_SAFETY_CACHE_TTL,
		helpers.HLS_REMUX_SAFETY_CACHE_SWEEP,
	)

	// Cache extracted WebVTT payloads to avoid repeated subtitle conversion work.
	app.SubtitleVTTCache = cache.New(helpers.SUBTITLE_CACHE_TTL, helpers.SUBTITLE_CACHE_CLEANUP)
	app.RoomHLSTombstone = cache.New(helpers.HLS_SESSION_TTL, helpers.HLS_SESSION_CACHE_SWEEP)

	if app.Settings.TmdbKey.Valid {
		tmdb, err := tmdb.New(app.Settings.TmdbKey.String)
		if err != nil {
			app.Logger.Warn("failed to initialize tmdb client", "error", err)
		} else {
			app.Tmdb = tmdb
			app.Logger.Info("tmdb client initialized successfully")
		}
	}

	if app.Settings.SpotifyClientID.Valid && app.Settings.SpotifyClientID.String != "" &&
		app.Settings.SpotifyClientSecret.Valid && app.Settings.SpotifyClientSecret.String != "" {
		spotifyClient, err := spotify.New(
			ctx,
			app.Settings.SpotifyClientID.String,
			app.Settings.SpotifyClientSecret.String,
		)

		if err != nil {
			app.Logger.Warn("failed to initialize spotify client", "error", err)
		} else {
			app.Spotify = spotifyClient
			app.Logger.Info("spotify client initialized successfully")
		}
	}

	if app.Settings.MoviesDir.Valid && app.Settings.MoviesDir.String != "" {
		go app.ScanMoviesLibrary()
	}

	if app.Settings.MusicDir.Valid && app.Settings.MusicDir.String != "" {
		go app.ScanMusicLibrary()
	}

	app.InitRouter()

	return &app, nil
}

func (app *Application) InitDB() error {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = helpers.DEFAULT_DB_PATH
	}

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

	if err := ensureDatabasePathWritable(dbPath, err == nil); err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	err = db.Ping()
	if err != nil {
		return fmt.Errorf("failed to connect to database %s: %w", dbPath, err)
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return fmt.Errorf("failed to enable WAL journal mode for database %s: %w", dbPath, err)
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
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("failed to close database directory write check for %s (%s): %w", dbPath, dir, err)
	}

	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("failed to remove database directory write check file for %s (%s): %w", dbPath, dir, err)
	}

	if !databaseExists {
		return nil
	}

	f, err := os.OpenFile(dbPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("database file is not writable at %s: %w", dbPath, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close database write check for %s: %w", dbPath, err)
	}

	return nil
}

// InitTables applies the embedded schema.
func (app *Application) InitTables() error {
	_, err := app.DB.Exec(SQL)
	if err != nil {
		return err
	}

	app.Logger.Info("database tables initialized successfully")

	return nil
}

// InitSettings loads persisted settings or creates the first record from env defaults.
func (app *Application) InitSettings(ctx context.Context) error {
	settings, err := app.Queries.GetSettings(ctx)
	if err == nil {
		applyRuntimeSettingOverrides(&settings)
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
		logsDir = helpers.DEFAULT_LOGS_DIR
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = helpers.DEFAULT_STATIC_DIR
	}

	hardwareAccelerationDevice := os.Getenv("HARDWARE_ACCELERATION_DEVICE")
	if hardwareAccelerationDevice == "" {
		hardwareAccelerationDevice = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	}

	moviesDir := os.Getenv("MOVIES_DIR")
	if moviesDir == "" {
		moviesDir = helpers.DEFAULT_MOVIES_DIR
	}

	showsDir := os.Getenv("SHOWS_DIR")
	if showsDir == "" {
		showsDir = helpers.DEFAULT_SHOWS_DIR
	}

	musicDir := os.Getenv("MUSIC_DIR")
	if musicDir == "" {
		musicDir = helpers.DEFAULT_MUSIC_DIR
	}

	params := database.CreateSettingsParams{
		TmdbKey:                    helpers.NullString(os.Getenv("TMDB_API_KEY")),
		JellyfinToken:              helpers.NullString(os.Getenv("JELLYFIN_TOKEN")),
		SpotifyClientID:            helpers.NullString(os.Getenv("SPOTIFY_CLIENT_ID")),
		SpotifyClientSecret:        helpers.NullString(os.Getenv("SPOTIFY_CLIENT_SECRET")),
		HardwareAccelerationDevice: helpers.NullString(hardwareAccelerationDevice),
		EnableLogger:               enableLogger,
		EnableWatcher:              enableWatcher,
		DownloadImages:             downloadImages,
		MoviesDir:                  helpers.NullString(moviesDir),
		ShowsDir:                   helpers.NullString(showsDir),
		MusicDir:                   helpers.NullString(musicDir),
		StaticDir:                  staticDir,
		LogsDir:                    logsDir,
	}

	settings, err = app.Queries.CreateSettings(ctx, params)
	if err != nil {
		return err
	}

	app.Logger.Info("default settings created successfully")

	applyRuntimeSettingOverrides(&settings)
	app.Settings = &settings

	return nil
}

// InitDirs ensures required app directories exist and creates configured media roots.
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

	if app.Settings.LogsDir != "" {
		created, err = helpers.GetOrCreateDir(app.Settings.LogsDir)
		if err != nil {
			return fmt.Errorf("failed to initialize logs directory: %w", err)
		}

		if created {
			app.Logger.Info("created logs directory", "path", app.Settings.LogsDir)
		}
	}

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

// InitLogger configures stdout logging for debug mode and file-backed logging otherwise.
func (app *Application) InitLogger() error {
	debug := os.Getenv("DEBUG") == "true"
	logToStdout := envBool(helpers.ENV_LOG_TO_STDOUT, debug)

	logsDir := os.Getenv("LOGS_DIR")
	if logsDir == "" {
		logsDir = helpers.DEFAULT_LOGS_DIR
	}

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

// InitDefaultUser bootstraps the first admin account on an empty database.
func (app *Application) InitDefaultUser(ctx context.Context) error {
	_, err := app.Queries.GetAdminUser(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	app.Logger.Info("no admin user found, creating default admin user...")

	name := strings.TrimSpace(os.Getenv(helpers.ENV_DEFAULT_ADMIN_NAME))
	if name == "" {
		name = helpers.DEFAULT_ADMIN_NAME
	}

	email := strings.TrimSpace(os.Getenv(helpers.ENV_DEFAULT_ADMIN_EMAIL))
	if email == "" {
		email = helpers.DEFAULT_ADMIN_EMAIL
	}

	password := strings.TrimSpace(os.Getenv(helpers.ENV_DEFAULT_ADMIN_PASSWORD))
	if password == "" {
		password = helpers.DEFAULT_ADMIN_PASSWORD
	}

	hashedPassword, err := helpers.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	params := database.CreateUserParams{
		Name:     name,
		Email:    email,
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

// InitSession stores sessions in SQLite so auth state survives process restarts.
func (app *Application) InitSession() {
	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(app.DB)
	sessionManager.Lifetime = 30 * 24 * time.Hour
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = envBool(helpers.ENV_SESSION_COOKIE_SECURE, false)

	app.SessionManager = sessionManager

	app.Logger.Info("session manager initialized successfully", "cookie_secure", sessionManager.Cookie.Secure)
}

func applyRuntimeSettingOverrides(settings *database.Setting) {
	if settings == nil {
		return
	}

	overrideNullStringSetting(&settings.TmdbKey, "TMDB_API_KEY")
	overrideNullStringSetting(&settings.JellyfinToken, "JELLYFIN_TOKEN")
	overrideNullStringSetting(&settings.SpotifyClientID, "SPOTIFY_CLIENT_ID")
	overrideNullStringSetting(&settings.SpotifyClientSecret, "SPOTIFY_CLIENT_SECRET")
	overrideNullStringSetting(&settings.HardwareAccelerationDevice, "HARDWARE_ACCELERATION_DEVICE")
	overrideBoolSetting(&settings.EnableLogger, "ENABLE_LOGGER")
	overrideBoolSetting(&settings.EnableWatcher, "ENABLE_WATCHER")
	overrideBoolSetting(&settings.DownloadImages, "DOWNLOAD_IMAGES")
	overrideNullStringSetting(&settings.MoviesDir, "MOVIES_DIR")
	overrideNullStringSetting(&settings.ShowsDir, "SHOWS_DIR")
	overrideNullStringSetting(&settings.MusicDir, "MUSIC_DIR")
	overrideStringSetting(&settings.StaticDir, "STATIC_DIR")
	overrideStringSetting(&settings.LogsDir, "LOGS_DIR")
}

func overrideNullStringSetting(target *sql.NullString, envName string) {
	if target == nil {
		return
	}

	value, ok := os.LookupEnv(envName)
	if !ok || strings.TrimSpace(value) == "" {
		return
	}

	*target = helpers.NullString(value)
}

func overrideStringSetting(target *string, envName string) {
	if target == nil {
		return
	}

	value, ok := os.LookupEnv(envName)
	if !ok || strings.TrimSpace(value) == "" {
		return
	}

	*target = value
}

func overrideBoolSetting(target *bool, envName string) {
	if target == nil {
		return
	}

	value, ok := os.LookupEnv(envName)
	if !ok || strings.TrimSpace(value) == "" {
		return
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		slog.Warn("invalid boolean value for env var, keeping current value", "env", envName, "value", value)
		return
	}

	*target = parsed
}

func envBool(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		slog.Warn("invalid boolean value for env var, using fallback", "env", name, "value", value, "fallback", fallback)
		return fallback
	}

	return parsed
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
		r.Post("/auth/login", app.AuthenticateUser)

		r.Group(func(r chi.Router) {
			r.Use(app.IsAuth)

			r.Route("/auth", func(r chi.Router) {
				r.Get("/user", app.GetCurrentAuthUser)
				r.Delete("/logout", app.DestroySession)
			})

			r.Route("/user", func(r chi.Router) {
				r.Put("/name", app.UpdateUserName)
				r.Put("/email", app.UpdateUserEmail)
				r.Put("/password", app.UpdateUserPassword)
				r.Put("/avatar", app.UpdateUserAvatar)
				r.Post("/avatar/upload", app.UploadUserAvatar)
				r.Delete("/", app.DeleteUserAccount)
			})

			r.Get("/static/*", app.ServeStaticFiles)

			r.Route("/tmdb", func(r chi.Router) {
				r.Get("/movies/in-theaters", app.GetMoviesInTheaters)
				r.Get("/movies/{id}", app.GetMovieByTmdbID)
			})

			r.Get("/search", app.SearchAll)
			r.Route("/search", func(r chi.Router) {
				r.Get("/", app.SearchAll)
				r.Get("/movies", app.SearchMovies)
				r.Get("/albums", app.SearchAlbums)
				r.Get("/musicians", app.SearchMusicians)
				r.Get("/tracks", app.SearchTracks)
			})

			r.Route("/movies", func(r chi.Router) {
				r.Get("/latest", app.GetLatestMovies)
				r.Get("/library", app.GetMoviesLibrary)
				r.Get("/stats", app.GetMoviesStats)
				r.Get("/liked", app.GetLikedMovies)
				r.Get("/{id}/like-status", app.GetMovieLikeStatus)
				r.Get("/genres", app.GetMovieGenresList)
				r.Get("/genres/{genreId}/movies", app.GetMoviesByGenreLibrary)
				r.Route("/playlists", func(pr chi.Router) {
					pr.Get("/{id}/movies", app.GetMoviePlaylistMovies)
					pr.Post("/{id}/movies", app.AddMoviesToMoviePlaylist)
					pr.Delete("/{id}/movies/{movieId}", app.RemoveMovieFromMoviePlaylist)
					pr.Get("/", app.GetMoviePlaylists)
					pr.Post("/", app.CreateMoviePlaylist)
					pr.Get("/{id}", app.GetMoviePlaylist)
					pr.Put("/{id}", app.UpdateMoviePlaylist)
					pr.Delete("/{id}", app.DeleteMoviePlaylist)
				})
				r.Get("/details/{id}", app.GetMovieDetails)
				r.Get("/{id}/technical-details", app.GetMovieTechnicalDetails)
				r.Get("/{id}/watch-progress", app.GetMovieWatchProgress)
				r.Post("/{id}/like", app.ToggleLikeMovie)
				r.Put("/{id}/watch-progress", app.UpdateMovieWatchProgress)
				r.Delete("/{id}/watch-progress", app.DeleteMovieWatchProgress)
				r.Put("/{id}/watch-progress/watched", app.SetMovieWatched)
				r.Get("/{id}/hls/{profile}/playlist.m3u8", app.HLSManifest)
				r.Get("/{id}/hls/{profile}/{filename}", app.HLSSegment)
				r.Get("/{id}/stream", app.StreamMovie)
				r.Get("/{id}/subtitles/{trackIndex}/web.vtt", app.SubtitleWebVTT)

				r.With(app.RequireAdmin).Post("/{id}/tmdb-search", app.TmdbSearchMovies)
				r.With(app.RequireAdmin).Put("/{id}/identify", app.IdentifyMovie)
				r.With(app.RequireAdmin).Patch("/{id}", app.UpdateMovieMetadata)
				r.With(app.RequireAdmin).Delete("/{id}", app.DeleteMovie)
			})

			r.Get("/users", app.GetUsers)

			r.Route("/admin/users", func(r chi.Router) {
				r.Use(app.RequireAdmin)
				r.Get("/", app.AdminGetUsers)
				r.Post("/", app.AdminCreateUser)
				r.Patch("/{id}", app.AdminUpdateUser)
				r.Delete("/{id}", app.AdminDeleteUser)
				r.Put("/{id}/password", app.AdminResetUserPassword)
			})

			r.Route("/watch-rooms", func(r chi.Router) {
				r.Get("/", app.GetWatchRooms)
				r.Post("/", app.CreateWatchRoom)
				r.Get("/{id}", app.GetWatchRoom)
				r.Post("/{id}/join", app.JoinWatchRoom)
				r.Get("/{id}/ws", app.WatchRoomWebSocket)
				r.Get("/{id}/stream", app.StreamWatchRoomMovie)
				r.Get("/{id}/hls/playlist.m3u8", app.WatchRoomHLSManifest)
				r.Get("/{id}/hls/{filename}", app.WatchRoomHLSSegment)
				r.Delete("/{id}", app.DeleteWatchRoom)
			})

			r.Route("/settings", func(r chi.Router) {
				r.Get("/", app.GetSettings)
				r.With(app.RequireAdmin).Get("/general", app.GetGeneralSettings)
				r.With(app.RequireAdmin).Put("/general", app.UpdateGeneralSettings)
				r.Get("/playback", app.GetPlaybackSettings)
				r.Put("/playback", app.UpdatePlaybackSettings)
				r.With(app.RequireAdmin).Post("/scan/music", app.TriggerMusicScan)
				r.With(app.RequireAdmin).Post("/scan/movies", app.TriggerMovieScan)
			})

			r.Route("/music", func(r chi.Router) {
				r.Get("/stats", app.GetMusicStats)

				r.Route("/albums", func(r chi.Router) {
					r.Get("/", app.GetAlbumsAlphabetical)
					r.Get("/details/{id}", app.GetAlbumDetails)
					r.Get("/latest", app.GetLatestAlbums)
					r.With(app.RequireAdmin).Delete("/{id}", app.DeleteAlbum)
				})

				r.Route("/musicians", func(r chi.Router) {
					r.Get("/", app.GetMusiciansAlphabetical)
					r.Get("/{id}", app.GetMusicianDetails)
				})

				r.Route("/tracks", func(r chi.Router) {
					r.Get("/", app.GetTracksAlphabetical)
					r.Get("/shuffle", app.GetShuffleTracks)
					r.Get("/details/{id}", app.GetTrackByID)
					r.Get("/{id}/stream", app.StreamTrack)
					r.Post("/{id}/like", app.ToggleLikeTrack)
					r.Get("/liked", app.GetLikedTracks)
					r.Get("/liked-ids", app.GetLikedTrackIDsForUser)
				})

				r.Route("/playlists", func(r chi.Router) {
					r.Get("/", app.GetPlaylists)
					r.Post("/", app.CreatePlaylist)
					r.Get("/{id}", app.GetPlaylist)
					r.Put("/{id}", app.UpdatePlaylist)
					r.Delete("/{id}", app.DeletePlaylist)
					r.Get("/{id}/tracks", app.GetPlaylistTracks)
					r.Post("/{id}/tracks", app.AddTracksToPlaylist)
					r.Delete("/{id}/tracks/{trackId}", app.RemoveTrackFromPlaylist)
					r.Put("/{id}/tracks/reorder", app.ReorderPlaylistTracks)
					r.Get("/{id}/collaborators", app.GetPlaylistCollaborators)
					r.Post("/{id}/collaborators", app.AddCollaborator)
					r.Delete("/{id}/collaborators/{userId}", app.RemoveCollaborator)
				})

				r.Route("/user-stats", func(r chi.Router) {
					r.Post("/play", app.RecordPlayEvent)
					r.Get("/overview", app.GetUserListeningStats)
					r.Get("/top-tracks", app.GetUserTopTracks)
					r.Get("/top-musicians", app.GetUserTopMusicians)
					r.Get("/top-genres", app.GetUserTopGenres)
					r.Get("/top-albums", app.GetUserTopAlbums)
					r.Get("/recently-played", app.GetUserRecentlyPlayed)
				})
			})
		})
	})

	// Register SPA fallback after /api routes so API paths cannot be captured.
	router.Get("/*", app.ServeFrontend)

	app.Router = router
}

// cleanupStaleHLSTempDirs removes leftover igloo-hls-* directories from the
// system temp directory. These accumulate when the server is killed without a
// graceful shutdown (e.g., systemd SIGKILL escalation, crash, power loss).
func cleanupStaleHLSTempDirs(logger applogger.LoggerInterface) {
	pattern := filepath.Join(os.TempDir(), "igloo-hls-*")
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

func (app *Application) ListenForShutdown() {
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	signal.Stop(quit)

	app.Logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if app.Server != nil {
		err := app.Server.Shutdown(ctx)
		if err != nil {
			app.Logger.Error("failed to shutdown server", "error", err)
		}
	}

	app.Logger.Info("running clean up tasks...")

	if app.WatchRoomHub != nil {
		app.WatchRoomHub.Shutdown()
	}

	// Stop FFmpeg sessions before cleaning up the FFmpeg binary.
	if app.HLSSessionCache != nil {
		count := 0
		for _, item := range app.HLSSessionCache.Items() {
			if session, ok := item.Object.(*HLSSession); ok {
				cleanupHLSSession(session)
				count++
			}
		}
		if count > 0 {
			app.Logger.Info("cleaned up HLS sessions", "count", count)
		}
		app.HLSSessionCache.Flush()
	}

	// Background tasks may still need database and logger access.
	app.Wait.Wait()

	if app.RemuxSafetyCache != nil {
		app.RemuxSafetyCache.Flush()
	}
	if app.SubtitleVTTCache != nil {
		app.SubtitleVTTCache.Flush()
	}
	if app.RoomHLSTombstone != nil {
		app.RoomHLSTombstone.Flush()
	}
	if app.Spotify != nil {
		app.Spotify.ClearAllCaches()
	}
	if app.Tmdb != nil {
		app.Tmdb.ClearCache()
	}

	err := ffprobe.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffprobe", "error", err)
	}

	err = ffmpeg.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffmpeg", "error", err)
	}

	if app.DB != nil {
		err = app.DB.Close()
		if err != nil {
			app.Logger.Error("failed to close database", "error", err)
		}
	}

	// Close the logger last so prior cleanup failures can still be logged.
	if app.LoggerCloser != nil {
		err = app.LoggerCloser()
		if err != nil {
			log.Printf("failed to close logger: %v", err)
		}
	}

	os.Exit(0)
}
