package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/spotify"
	"igloo/cmd/internal/tmdb"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitApp() (*Application, error) {
	var app Application

	err := app.InitDB()
	if err != nil {
		return nil, err
	}

	err = app.InitSettings()
	if err != nil {
		return nil, err
	}

	if app.Settings.EnableLogger {
		l, err := logger.New(app.Settings.Debug, app.Settings.LogsDir)
		if err != nil {
			return nil, err
		}

		app.Logger = l
	}

	err = app.InitDefaultUser()
	if err != nil {
		return nil, err
	}

	if app.Settings.EnableWatcher {
		err = app.InitWatcher()
		if err != nil {
			return nil, err
		}
	}

	if app.Settings.FfprobePath != "" {
		f, err := ffprobe.New(app.Settings.FfprobePath)
		if err != nil {
			return nil, err
		}

		app.Ffprobe = f
	}

	if app.Settings.TmdbApiKey != "" {
		t, err := tmdb.New(app.Settings.TmdbApiKey)
		if err != nil {
			return nil, err
		}

		app.Tmdb = t
	}

	if app.Settings.SpotifyClientID != "" || app.Settings.SpotifyClientSecret != "" {
		s, err := spotify.New(app.Settings.SpotifyClientID, app.Settings.SpotifyClientSecret)
		if err != nil {
			return nil, err
		}

		app.Spotify = s
	}

	return &app, nil
}

func (app *Application) InitDB() error {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}

	port, err := strconv.Atoi(os.Getenv("POSTGRES_PORT"))
	if err != nil {
		port = 5432
	}

	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "postgres"
	}

	pwd := os.Getenv("POSTGRES_PASSWORD")
	if pwd == "" {
		pwd = "postgres"
	}

	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "igloo"
	}

	sslMode := os.Getenv("POSTGRES_SSL_MODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	maxCon, err := strconv.Atoi(os.Getenv("POSTGRES_MAX_CONNECTIONS"))
	if err != nil {
		maxCon = 10
	}

	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		user,
		pwd,
		host,
		port,
		dbName,
		sslMode,
	)

	dbConfig, err := pgxpool.ParseConfig(dbUrl)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}

	dbConfig.MaxConns = int32(maxCon)

	dbpool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create database pool: %w", err)
	}

	err = dbpool.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	queries := database.New(dbpool)

	app.Db = dbpool
	app.Queries = queries

	return nil
}

func (app *Application) InitSettings() error {
	settings, err := app.Queries.GetSettings(context.Background())
	if err != nil {
		var s database.CreateSettingsParams

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			port = 8080
		}
		s.Port = int32(port)

		debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
		if err != nil {
			debug = true
		}
		s.Debug = debug

		enableLogger, err := strconv.ParseBool(os.Getenv("ENABLE_LOGGER"))
		if err != nil {
			enableLogger = false
		}
		s.EnableLogger = enableLogger

		s.LogsDir = os.Getenv("LOGS_DIR")
		if s.LogsDir == "" {
			s.LogsDir = "logs"
		}

		enableWatcher, err := strconv.ParseBool(os.Getenv("ENABLE_WATCHER"))
		if err != nil {
			enableWatcher = false
		}
		s.EnableWatcher = enableWatcher

		downloadImages, err := strconv.ParseBool(os.Getenv("DOWNLOAD_IMAGES"))
		if err != nil {
			downloadImages = false
		}
		s.DownloadImages = downloadImages

		s.StaticDir = os.Getenv("STATIC_DIR")
		if s.StaticDir == "" {
			s.StaticDir = "static"
		}

		err = InitDirs(&s)
		if err != nil {
			log.Println("unable to create directoreies correctly")
		}

		enableTranscoding, err := strconv.ParseBool(os.Getenv("ENABLE_TRANSCODING"))
		if err != nil {
			enableTranscoding = false
		}
		s.EnableHardwareAcceleration = enableTranscoding

		s.HardwareAccelerationMethod = os.Getenv("HARDWARE_ACCELERATION_METHOD")
		if s.HardwareAccelerationMethod == "" {
			s.HardwareAccelerationMethod = "cpu"
		}

		s.FfmpegPath = os.Getenv("FFMPEG_PATH")
		s.FfprobePath = os.Getenv("FFPROBE_PATH")
		s.TmdbApiKey = os.Getenv("TMDB_API_KEY")
		s.JellyfinToken = os.Getenv("JELLYFIN_TOKEN")
		s.MoviesDir = os.Getenv("MOVIES_DIR")
		s.MusicDir = os.Getenv("MUSIC_DIR")
		s.TvshowsDir = os.Getenv("TVSHOWS_DIR")
		s.TranscodeDir = os.Getenv("TRANSCODE_DIR")
		s.MoviesImgDir = os.Getenv("MOVIES_IMG_DIR")
		s.StudiosImgDir = os.Getenv("STUDIOS_IMG_DIR")
		s.ArtistsImgDir = os.Getenv("ARTISTS_IMG_DIR")
		s.AvatarImgDir = os.Getenv("AVATAR_IMG_DIR")
		s.PlexToken = os.Getenv("PLEX_TOKEN")
		s.SpotifyClientID = os.Getenv("SPOTIFY_CLIENT_ID")
		s.SpotifyClientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")

		settings, err = app.Queries.CreateSettings(context.Background(), s)
		if err != nil {
			return fmt.Errorf("failed to create settings: %w", err)
		}
	}

	app.Settings = &settings

	return nil
}

func (app *Application) InitDefaultUser() error {
	count, err := app.Queries.GetTotalUsersCount(context.Background())
	if err != nil {
		return fmt.Errorf("unable to determine total users count: %w", err)
	}

	if count == 0 {
		hashPassword, err := helpers.HashPassword(DEFAULT_PASSWORD)
		if err != nil {
			return fmt.Errorf("failed to hash password for default user: %w", err)
		}

		_, err = app.Queries.CreateUser(context.Background(), database.CreateUserParams{
			Name:     "Admin User",
			Email:    "admin@example.com",
			Username: "admin",
			Password: hashPassword,
			IsActive: true,
			IsAdmin:  true,
		})

		if err != nil {
			return fmt.Errorf("failed to create default user: %w", err)
		}
	}

	return nil
}

func InitDirs(s *database.CreateSettingsParams) error {
	err := helpers.CreateDir(s.StaticDir)
	if err != nil {
		return err
	}

	if s.EnableLogger {
		err = helpers.CreateDir(s.LogsDir)
		if err != nil {
			return err
		}
	}

	imgDir := filepath.Join(s.StaticDir, "images")
	err = helpers.CreateDir(imgDir)
	if err != nil {
		return err
	}

	moviesImgDir := filepath.Join(imgDir, "movies")
	err = helpers.CreateDir(moviesImgDir)
	if err != nil {
		return err
	}

	studiosImgDir := filepath.Join(imgDir, "studios")
	err = helpers.CreateDir(studiosImgDir)
	if err != nil {
		return err
	}

	artistsImgDir := filepath.Join(imgDir, "artists")
	err = helpers.CreateDir(artistsImgDir)
	if err != nil {
		return err
	}

	avatarImgDir := filepath.Join(imgDir, "avatar")
	err = helpers.CreateDir(avatarImgDir)
	if err != nil {
		return err
	}

	s.MoviesImgDir = moviesImgDir
	s.StudiosImgDir = studiosImgDir
	s.ArtistsImgDir = artistsImgDir
	s.AvatarImgDir = avatarImgDir

	return nil
}

func (app *Application) InitWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %v", err)
	}

	if app.Settings.MoviesDir != "" {
		app.Logger.Info(fmt.Sprintf("will watch %s for movies", app.Settings.MoviesDir))
	}

	if app.Settings.TvshowsDir != "" {
		app.Logger.Info(fmt.Sprintf("will watch %s for music", app.Settings.TvshowsDir))
	}

	app.Watcher = w

	return nil
}

func (app *Application) InitRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)

	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		r.Route("/movies", func(r chi.Router) {
			r.Get("/count", app.getTotalMovieCount) // GET /api/v1/movies/count
			r.Get("/latest", app.getLatestMovies)   // GET /api/v1/movies/latest
			r.Get("/", app.getMoviesPaginated)      // GET /api/v1/movies?page=1&limit=24
			r.Post("/", app.createTmdbMovie)        // POST /api/v1/movies
			r.Get("/{id}", app.getMovieDetails)     // GET /api/v1/movies/{id}
		})
	})

	return router
}
