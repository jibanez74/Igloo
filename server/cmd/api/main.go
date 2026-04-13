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

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for database/sql
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

var movieMetadataLockColumns = []string{
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
	SubtitleVTTCache *cache.Cache
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

	// Load local development settings from server/.env before startup.
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

	// Cache extracted WebVTT payloads to avoid repeated subtitle conversion work.
	app.SubtitleVTTCache = cache.New(helpers.SUBTITLE_CACHE_TTL, helpers.SUBTITLE_CACHE_CLEANUP)

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

	db, err := sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
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

// InitTables applies the embedded schema and any small startup migrations.
func (app *Application) InitTables() error {
	_, err := app.DB.Exec(SQL)
	if err != nil {
		return err
	}

	err = app.ensureMovieMetadataLockColumns()
	if err != nil {
		return err
	}

	app.Logger.Info("database tables initialized successfully")

	return nil
}

func (app *Application) ensureMovieMetadataLockColumns() error {
	rows, err := app.DB.Query("PRAGMA table_info(movies)")
	if err != nil {
		return err
	}
	defer rows.Close()

	existingColumns := make(map[string]bool)

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int

		err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk)
		if err != nil {
			return err
		}

		existingColumns[name] = true
	}

	err = rows.Err()
	if err != nil {
		return err
	}

	for _, columnName := range movieMetadataLockColumns {
		if existingColumns[columnName] {
			continue
		}

		statement := fmt.Sprintf(
			"ALTER TABLE movies ADD COLUMN %s BOOLEAN NOT NULL DEFAULT false",
			columnName,
		)
		_, err = app.DB.Exec(statement)
		if err != nil {
			return err
		}
	}

	return nil
}

// InitSettings loads persisted settings or creates the first record from env defaults.
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

	hardwareAccelerationDevice := os.Getenv("HARDWARE_ACCELERATION_DEVICE")
	if hardwareAccelerationDevice == "" {
		hardwareAccelerationDevice = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
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

	logsDir := os.Getenv("LOGS_DIR")
	if logsDir == "" {
		logsDir = "logs"
	}

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
		// Public endpoints — no authentication required.
		r.Get("/health", app.HealthCheck)
		r.Post("/auth/login", app.AuthenticateUser)

		// All remaining /api routes require a valid session.
		r.Group(func(r chi.Router) {
			r.Use(app.IsAuth)

			r.Route("/auth", func(r chi.Router) {
				r.Get("/user", app.GetCurrentAuthUser)
				r.Delete("/logout", app.DestroySession)
			})

			r.Route("/user", func(r chi.Router) {
				r.Put("/name", app.UpdateUserName)
				r.Put("/password", app.UpdateUserPassword)
				r.Put("/avatar", app.UpdateUserAvatar)
				r.Post("/avatar/upload", app.UploadUserAvatar)
				r.Delete("/", app.DeleteUserAccount)
			})

			// Static assets include uploaded avatars and scanner-downloaded artwork.
			r.Get("/static/*", app.ServeStaticFiles)

			r.Route("/tmdb", func(r chi.Router) {
				r.Get("/movies/in-theaters", app.GetMoviesInTheaters)
				r.Get("/movies/{id}", app.GetMovieByTmdbID)
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

				// Admin-only: library management operations.
				r.With(app.RequireAdmin).Post("/{id}/tmdb-search", app.TmdbSearchMovies)
				r.With(app.RequireAdmin).Put("/{id}/identify", app.IdentifyMovie)
				r.With(app.RequireAdmin).Patch("/{id}", app.UpdateMovieMetadata)
				r.With(app.RequireAdmin).Delete("/{id}", app.DeleteMovie)
			})

			r.Route("/settings", func(r chi.Router) {
				r.Get("/", app.GetSettings)
				// Admin-only: scan triggers mutate the library.
				r.With(app.RequireAdmin).Post("/scan/music", app.TriggerMusicScan)
				r.With(app.RequireAdmin).Post("/scan/movies", app.TriggerMovieScan)
			})

			r.Route("/music", func(r chi.Router) {
				r.Get("/stats", app.GetMusicStats)

				r.Route("/albums", func(r chi.Router) {
					r.Get("/", app.GetAlbumsAlphabetical)
					r.Get("/details/{id}", app.GetAlbumDetails)
					r.Get("/latest", app.GetLatestAlbums)
					// Admin-only: destructive library operation.
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
					r.Get("/liked", app.GetLikedTrackIDs)
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

	// Frontend routes - serve the React SPA
	// This must be registered after /api routes to avoid conflicts
	// All non-API routes fall through to the SPA (client-side routing)
	router.Get("/*", app.ServeFrontend)

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
	// Gives in-flight requests 30 seconds to complete.
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

	// Clean up all HLS sessions (kill FFmpeg, delete temp dirs) before FFmpeg binary cleanup.
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

	// Wait for any in-flight background tasks to complete.
	// These may still need database and logger access.
	app.Wait.Wait()

	// Clean up ffprobe temp directory and extracted binary.
	err := ffprobe.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffprobe", "error", err)
	}

	// Clean up ffmpeg temp directory and extracted binary.
	err = ffmpeg.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffmpeg", "error", err)
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
