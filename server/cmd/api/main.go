package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
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
	"igloo/sqlc"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for database/sql
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type Application struct {
	DB                   *sql.DB
	Queries              *database.Queries
	Settings             *database.Setting
	Config               RuntimeConfig
	Logger               applogger.LoggerInterface
	LoggerCloser         func() error
	Ffprobe              ffprobe.FfprobeInterface
	FFmpeg               ffmpeg.FFmpegInterface
	Spotify              spotify.SpotifyInterface
	Tmdb                 tmdb.TmdbInterface
	TmdbImageBaseURL     string
	TmdbImageHTTPClient  *http.Client
	SessionManager       *scs.SessionManager
	Wait                 *sync.WaitGroup
	Router               *chi.Mux
	Server               *http.Server
	ScannerDBMu          sync.Mutex
	SearchVocab          searchVocabCache
	HLSSessionCache      *cache.Cache
	HLSSessionGroup      singleflight.Group
	HLSTranscodeLimiter  *hlsTranscodeLimiter
	PersonalHLSMu        sync.Mutex
	RemuxSafetyCache     *cache.Cache
	SubtitleVTTCache     *cache.Cache
	SubtitleExtractGroup singleflight.Group
	RoomHLSTombstone     *cache.Cache
	RoomHLSMu            sync.Mutex
	WatchRoomHub         *WatchRoomHub
	QuickConnect         *QuickConnectBroker
	AuthLimiter          *rateLimiter
	DeviceLastSeen       *cache.Cache
}

//go:embed all:webdist
var FrontendFS embed.FS

func main() {
	log.Println("igloo server starting up...")

	envFile, loadedEnvFile, err := LoadRuntimeEnvFile()
	if err != nil {
		log.Printf("warning: %v", err)
	}

	if loadedEnvFile {
		log.Printf("loaded environment file %s", envFile)
	}

	app, err := InitApp()
	if err != nil {
		log.Fatal(err)
	}

	app.Server = &http.Server{
		Addr:    fmt.Sprintf(":%d", app.Config.Port),
		Handler: app.Router,
	}

	go app.ListenForShutdown()

	log.Printf("server listening on port %d", app.Config.Port)

	err = app.Server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		// InitApp already extracted the ffmpeg/ffprobe binaries and opened the
		// database and logger, so a serve failure (e.g. port already in use) must
		// run cleanup rather than exit bare.
		app.Logger.Error("server failed to start", "error", err)
		app.cleanupMediaBinaries()
		app.closeDatabase()
		app.closeLogger()
		os.Exit(1)
	}
}

func InitApp() (*Application, error) {
	config, err := NewRuntimeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime config: %v", err)
	}

	app := Application{
		Config: config,
		Wait:   &sync.WaitGroup{},
	}

	ctx := context.Background()

	// Logger comes first so later startup failures are captured consistently.
	err = app.InitLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	err = app.InitDB()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	err = app.InitTables()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database tables: %v", err)
	}

	app.Queries, err = database.Prepare(ctx, app.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare database queries: %v", err)
	}

	err = app.InitSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize settings: %v", err)
	}

	// Remove any leftover HLS temp directories from a previous run that did not
	// shut down cleanly (crash, SIGKILL from systemd, power loss, etc.).
	cleanupStaleHLSTempDirs(app.Logger, app.Settings.TranscodeDir)

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

	app.initRuntimeCaches()
	app.ScanMoviesLibrary()
	app.MusicScanLibrary()

	app.InitRouter()

	return &app, nil
}

func (app *Application) initRuntimeCaches() {
	app.HLSTranscodeLimiter = newHLSTranscodeLimiter(configuredHLSMaxCPUTranscodes())

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

	app.QuickConnect = NewQuickConnectBroker()
	app.AuthLimiter = newRateLimiter()

	// Throttles devices.last_used_at writes to at most one per device per TTL.
	app.DeviceLastSeen = cache.New(deviceLastSeenTTL, deviceLastSeenTTL)
}

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

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close database write check for %s: %w", dbPath, err)
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

	downloadImages, _ := strconv.ParseBool(os.Getenv("DOWNLOAD_IMAGES"))
	enableWatcher, _ := strconv.ParseBool(os.Getenv("ENABLE_WATCHER"))

	staticDir := configuredStaticDir()
	transcodeDir := configuredTranscodeDir()

	hardwareAccelerationDevice := os.Getenv("HARDWARE_ACCELERATION_DEVICE")
	if hardwareAccelerationDevice == "" {
		hardwareAccelerationDevice = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	}

	params := database.CreateSettingsParams{
		TmdbKey:                    helpers.NullString(os.Getenv("TMDB_API_KEY")),
		JellyfinApiKey:             helpers.NullString(os.Getenv("JELLYFIN_API_KEY")),
		SpotifyClientID:            helpers.NullString(os.Getenv("SPOTIFY_CLIENT_ID")),
		SpotifyClientSecret:        helpers.NullString(os.Getenv("SPOTIFY_CLIENT_SECRET")),
		HardwareAccelerationDevice: helpers.NullString(hardwareAccelerationDevice),
		EnableWatcher:              enableWatcher,
		DownloadImages:             downloadImages,
		MoviesDir:                  optionalEnvSetting("MOVIES_DIR"),
		ShowsDir:                   optionalEnvSetting("SHOWS_DIR"),
		MusicDir:                   optionalEnvSetting("MUSIC_DIR"),
		StaticDir:                  staticDir,
		TranscodeDir:               transcodeDir,
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
	if err := validateExistingDir(dir.String); err != nil {
		app.Logger.Warn("disabling inaccessible media directory", "type", mediaType, "path", dir.String, "error", err)
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
	debug := envBool("DEBUG", app.Config.Debug)
	logToStdout := envBool(helpers.ENV_LOG_TO_STDOUT, app.Config.LogToStdout || debug)

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

func (app *Application) InitSession() {
	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(app.DB)
	sessionManager.Lifetime = 30 * 24 * time.Hour
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = envBool(helpers.ENV_SESSION_COOKIE_SECURE, app.Config.SessionCookieSecure)

	app.SessionManager = sessionManager

	app.Logger.Info("session manager initialized successfully", "cookie_secure", sessionManager.Cookie.Secure)
}

func optionalEnvSetting(envName string) sql.NullString {
	return helpers.NullString(strings.TrimSpace(os.Getenv(envName)))
}

func (app *Application) InitRouter() {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(preserveClientSocketIP)
	router.Use(middleware.RealIP)
	router.Use(app.RequestLogger)
	router.Use(middleware.Recoverer)

	app.registerWebSocketRoutes(router)

	router.Group(func(r chi.Router) {
		r.Use(app.LoadAndSaveSession)
		app.registerSessionRoutes(r)
	})

	app.Router = router
}

func (app *Application) registerWebSocketRoutes(router chi.Router) {
	router.With(app.DeviceTokenAuth, app.LoadSessionReadOnly, app.IsAuth).Get("/api/watch-rooms/{id}/ws", app.WatchRoomWebSocket)
}

func (app *Application) registerSessionRoutes(r chi.Router) {
	app.registerAPIRoutes(r)

	// Register SPA fallback after /api routes so API paths cannot be captured.
	r.Get("/*", app.ServeFrontend)
}

func (app *Application) registerAPIRoutes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", app.HealthCheck)
		r.Post("/auth/login", app.AuthenticateUser)
		r.Post("/auth/device-login", app.AuthenticateDevice)
		r.With(app.DeviceTokenAuth).Get("/auth/user", app.GetCurrentAuthUser)
		r.Post("/quick-connect/initiate", app.InitiateQuickConnect)
		r.Post("/quick-connect/redeem", app.RedeemQuickConnect)
		app.registerAuthenticatedAPIRoutes(r)
	})
}

func (app *Application) registerAuthenticatedAPIRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(app.DeviceTokenAuth)
		r.Use(app.IsAuth)

		app.registerAuthRoutes(r)
		app.registerDeviceRoutes(r)
		app.registerUserRoutes(r)
		app.registerNotificationRoutes(r)
		r.Get("/static/*", app.ServeStaticFiles)
		app.registerTMDBRoutes(r)
		app.registerSpotifyRoutes(r)
		app.registerSearchRoutes(r)
		app.registerMovieRoutes(r)
		r.Get("/users", app.GetUsers)
		app.registerAdminUserRoutes(r)
		app.registerWatchRoomRoutes(r)
		app.registerSettingsRoutes(r)
		app.registerMusicRoutes(r)
	})
}

func (app *Application) registerAuthRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Delete("/logout", app.DestroySession)
	})
}

func (app *Application) registerDeviceRoutes(r chi.Router) {
	r.Post("/quick-connect/approve", app.ApproveQuickConnect)

	r.Route("/devices", func(r chi.Router) {
		r.Get("/", app.GetDevices)
		r.Patch("/{id}", app.RenameDevice)
		r.Delete("/{id}", app.RevokeDevice)
	})
}

func (app *Application) registerUserRoutes(r chi.Router) {
	r.Route("/user", func(r chi.Router) {
		r.Put("/name", app.UpdateUserName)
		r.Put("/email", app.UpdateUserEmail)
		r.Put("/password", app.UpdateUserPassword)
		r.Put("/avatar", app.UpdateUserAvatar)
		r.Post("/avatar/upload", app.UploadUserAvatar)
		r.Delete("/", app.DeleteUserAccount)
	})
}

func (app *Application) registerNotificationRoutes(r chi.Router) {
	r.Route("/notifications", func(r chi.Router) {
		r.Get("/", app.ListNotifications)
		r.Post("/", app.CreateNotification)
		r.Get("/unread-count", app.GetUnreadNotificationCount)
		r.Post("/read-all", app.MarkAllNotificationsRead)
		r.Post("/{id}/read", app.MarkNotificationRead)
		r.Delete("/{id}", app.DeleteNotification)
	})
}

func (app *Application) registerTMDBRoutes(r chi.Router) {
	r.Route("/tmdb", func(r chi.Router) {
		r.Get("/status", app.GetTmdbStatus)
		r.Get("/images/{size}/{file}", app.ProxyTmdbImage)
		r.Post("/movies/search", app.SearchTmdbMovies)
		r.Get("/movies/in-theaters", app.GetMoviesInTheaters)
		r.Get("/movies/{id}", app.GetMovieByTmdbID)
	})
}

func (app *Application) registerSpotifyRoutes(r chi.Router) {
	r.Route("/spotify", func(r chi.Router) {
		r.Get("/status", app.GetSpotifyStatus)
		r.Post("/albums/search", app.SearchSpotifyAlbums)
		r.Post("/tracks/search", app.SearchSpotifyTracks)
	})
}

func (app *Application) registerSearchRoutes(r chi.Router) {
	r.Get("/search", app.SearchAll)
	r.Route("/search", func(r chi.Router) {
		r.Get("/", app.SearchAll)
		r.Get("/movies", app.SearchMovies)
		r.Get("/albums", app.SearchAlbums)
		r.Get("/musicians", app.SearchMusicians)
		r.Get("/tracks", app.SearchTracks)
	})
}

func (app *Application) registerMovieRoutes(r chi.Router) {
	r.Route("/movies", func(r chi.Router) {
		r.Get("/latest", app.GetLatestMovies)
		r.Get("/library", app.GetMoviesLibrary)
		r.Get("/stats", app.GetMoviesStats)
		r.Get("/liked", app.GetLikedMovies)
		r.Get("/{id}/like-status", app.GetMovieLikeStatus)
		r.Get("/genres", app.GetMovieGenresList)
		r.Get("/genres/{genreId}/movies", app.GetMoviesByGenreLibrary)
		app.registerMoviePlaylistRoutes(r)
		r.Get("/details/{id}", app.GetMovieDetails)
		r.Get("/{id}/technical-details", app.GetMovieTechnicalDetails)
		r.Get("/{id}/watch-progress", app.GetMovieWatchProgress)
		r.Post("/{id}/like", app.ToggleLikeMovie)
		r.Put("/{id}/watch-progress", app.UpdateMovieWatchProgress)
		r.Delete("/{id}/watch-progress", app.DeleteMovieWatchProgress)
		r.Put("/{id}/watch-progress/watched", app.SetMovieWatched)
		r.Post("/{id}/hls/session/stop", app.StopPersonalHLSSession)
		r.Get("/{id}/hls/{profile}/playlist.m3u8", app.HLSManifest)
		r.Get("/{id}/hls/{profile}/{filename}", app.HLSSegment)
		r.Get("/{id}/stream", app.StreamMovie)
		r.Get("/{id}/subtitles/{trackIndex}/web.vtt", app.SubtitleWebVTT)

		r.With(app.RequireAdmin).Post("/{id}/tmdb-search", app.TmdbSearchMovies)
		r.With(app.RequireAdmin).Put("/{id}/identify", app.IdentifyMovie)
		r.With(app.RequireAdmin).Patch("/{id}", app.UpdateMovieMetadata)
		r.With(app.RequireAdmin).Delete("/{id}", app.DeleteMovie)
	})
}

func (app *Application) registerMoviePlaylistRoutes(r chi.Router) {
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
}

func (app *Application) registerAdminUserRoutes(r chi.Router) {
	r.Route("/admin/users", func(r chi.Router) {
		r.Use(app.RequireAdmin)
		r.Get("/", app.AdminGetUsers)
		r.Post("/", app.AdminCreateUser)
		r.Patch("/{id}", app.AdminUpdateUser)
		r.Delete("/{id}", app.AdminDeleteUser)
		r.Put("/{id}/password", app.AdminResetUserPassword)
	})
}

func (app *Application) registerWatchRoomRoutes(r chi.Router) {
	r.Route("/watch-rooms", func(r chi.Router) {
		r.Get("/", app.GetWatchRooms)
		r.Post("/", app.CreateWatchRoom)
		r.Get("/{id}", app.GetWatchRoom)
		r.Post("/{id}/join", app.JoinWatchRoom)
		r.Get("/{id}/stream", app.StreamWatchRoomMovie)
		r.Get("/{id}/hls/playlist.m3u8", app.WatchRoomHLSManifest)
		r.Get("/{id}/hls/{filename}", app.WatchRoomHLSSegment)
		r.Delete("/{id}", app.DeleteWatchRoom)
	})
}

func (app *Application) registerSettingsRoutes(r chi.Router) {
	r.Route("/settings", func(r chi.Router) {
		r.Get("/", app.GetSettings)
		r.With(app.RequireAdmin).Get("/general", app.GetGeneralSettings)
		r.With(app.RequireAdmin).Put("/general", app.UpdateGeneralSettings)
		r.With(app.RequireAdmin).Put("/libraries", app.UpdateLibrarySettings)
		r.Get("/playback", app.GetPlaybackSettings)
		r.Put("/playback", app.UpdatePlaybackSettings)
		r.With(app.RequireAdmin).Post("/scan/music", app.TriggerMusicScan)
		r.With(app.RequireAdmin).Post("/scan/movies", app.TriggerMovieScan)
	})
}

func (app *Application) registerMusicRoutes(r chi.Router) {
	r.Route("/music", func(r chi.Router) {
		r.Get("/stats", app.GetMusicStats)
		app.registerAlbumRoutes(r)
		app.registerMusicianRoutes(r)
		app.registerTrackRoutes(r)
		app.registerPlaylistRoutes(r)
		app.registerUserStatsRoutes(r)
	})
}

func (app *Application) registerAlbumRoutes(r chi.Router) {
	r.Route("/albums", func(r chi.Router) {
		r.Get("/", app.GetAlbumsAlphabetical)
		r.Get("/details/{id}", app.GetAlbumDetails)
		r.Get("/latest", app.GetLatestAlbums)
		r.With(app.RequireAdmin).Delete("/{id}", app.DeleteAlbum)
	})
}

func (app *Application) registerMusicianRoutes(r chi.Router) {
	r.Route("/musicians", func(r chi.Router) {
		r.Get("/", app.GetMusiciansAlphabetical)
		r.Get("/{id}", app.GetMusicianDetails)
	})
}

func (app *Application) registerTrackRoutes(r chi.Router) {
	r.Route("/tracks", func(r chi.Router) {
		r.Get("/", app.GetTracksAlphabetical)
		r.Get("/shuffle", app.GetShuffleTracks)
		r.Get("/details/{id}", app.GetTrackByID)
		r.Get("/{id}/stream", app.StreamTrack)
		r.Post("/{id}/like", app.ToggleLikeTrack)
		r.Get("/liked", app.GetLikedTracks)
		r.Get("/liked-ids", app.GetLikedTrackIDsForUser)
	})
}

func (app *Application) registerPlaylistRoutes(r chi.Router) {
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
}

func (app *Application) registerUserStatsRoutes(r chi.Router) {
	r.Route("/user-stats", func(r chi.Router) {
		r.Post("/play", app.RecordPlayEvent)
		r.Get("/overview", app.GetUserListeningStats)
		r.Get("/top-tracks", app.GetUserTopTracks)
		r.Get("/top-musicians", app.GetUserTopMusicians)
		r.Get("/top-genres", app.GetUserTopGenres)
		r.Get("/top-albums", app.GetUserTopAlbums)
		r.Get("/recently-played", app.GetUserRecentlyPlayed)
	})
}

func cleanupStaleHLSTempDirs(logger applogger.LoggerInterface, transcodeDir string) {
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

func (app *Application) ListenForShutdown() {
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	signal.Stop(quit)

	app.Logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	app.shutdownHTTPServer(ctx)

	app.Logger.Info("running clean up tasks...")

	app.shutdownWatchRoomHub()
	app.cleanupHLSSessions()

	// Background tasks may still need database and logger access.
	app.Wait.Wait()

	app.flushRuntimeCaches()
	app.clearMediaClientCaches()
	app.cleanupMediaBinaries()
	app.closeDatabase()
	app.closeLogger()

	os.Exit(0)
}

func (app *Application) shutdownHTTPServer(ctx context.Context) {
	if app.Server == nil {
		return
	}

	err := app.Server.Shutdown(ctx)
	if err != nil {
		app.Logger.Error("failed to shutdown server", "error", err)
	}
}

func (app *Application) shutdownWatchRoomHub() {
	if app.WatchRoomHub != nil {
		app.WatchRoomHub.Shutdown()
	}
}

func (app *Application) cleanupHLSSessions() {
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
}

func (app *Application) flushRuntimeCaches() {
	if app.RemuxSafetyCache != nil {
		app.RemuxSafetyCache.Flush()
	}
	if app.SubtitleVTTCache != nil {
		app.SubtitleVTTCache.Flush()
	}
	if app.RoomHLSTombstone != nil {
		app.RoomHLSTombstone.Flush()
	}
}

func (app *Application) clearMediaClientCaches() {
	if app.Spotify != nil {
		app.Spotify.ClearAllCaches()
	}
	if app.Tmdb != nil {
		app.Tmdb.ClearCache()
	}
}

func (app *Application) cleanupMediaBinaries() {
	err := ffprobe.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffprobe", "error", err)
	}

	err = ffmpeg.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffmpeg", "error", err)
	}
}

func (app *Application) closeDatabase() {
	// Close prepared statements before the connection that owns them.
	if app.Queries != nil {
		err := app.Queries.Close()
		if err != nil {
			app.Logger.Error("failed to close prepared statements", "error", err)
		}
	}

	if app.DB != nil {
		err := app.DB.Close()
		if err != nil {
			app.Logger.Error("failed to close database", "error", err)
		}
	}
}

func (app *Application) closeLogger() {
	// Close the logger last so prior cleanup failures can still be logged.
	if app.LoggerCloser != nil {
		err := app.LoggerCloser()
		if err != nil {
			log.Printf("failed to close logger: %v", err)
		}
	}
}
