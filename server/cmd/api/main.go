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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Failed to load .env file: ", err)
	}

	app, err := InitApp()
	if err != nil {
		log.Fatal("unable to create app", err)
	}

	err = http.ListenAndServe(fmt.Sprintf(":%d", app.Settings.Port), app.InitRouter())
	if err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}

func InitApp() (*Application, error) {
	var app Application

	err := app.InitDB()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	err = app.InitSettings(ctx)
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

	err = app.InitDefaultUser(ctx)
	if err != nil {
		return nil, err
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
		return err
	}

	dbConfig.MaxConns = int32(maxCon)

	dbpool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return err
	}

	err = dbpool.Ping(context.Background())
	if err != nil {
		return err
	}

	queries := database.New(dbpool)

	app.Db = dbpool
	app.Queries = queries

	return nil
}

func (app *Application) InitSettings(ctx context.Context) error {
	settings, err := app.Queries.GetSettings(ctx)
	if err != nil {
		var s database.CreateSettingsParams

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err != nil {
			log.Println("fail to get PORT value from environment variables.  Will default to PORT 8080.")
			port = 8080
		}
		s.Port = int32(port)

		debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
		if err != nil {
			log.Println("Fail to get DEBUG value from environment variables.  Debug mode will be turned off.")
			debug = true
		}
		s.Debug = debug

		enableLogger, err := strconv.ParseBool(os.Getenv("ENABLE_LOGGER"))
		if err != nil {
			log.Println("Fail to get ENABLE_LOGGER value from environment variables.  Log messages will be disabled.")
			enableLogger = false
		}
		s.EnableLogger = enableLogger

		enableWatcher, err := strconv.ParseBool(os.Getenv("ENABLE_WATCHER"))
		if err != nil {
			log.Println("Fail to get ENABLE_WATCHER value from environment variables.  File monitoring for media directories will be disabled.")
			enableWatcher = false
		}
		s.EnableWatcher = enableWatcher

		downloadImages, err := strconv.ParseBool(os.Getenv("DOWNLOAD_IMAGES"))
		if err != nil {
			log.Println("Fail to get DOWNLOAD_IMAGES value from environment variables.  Defaulting to use remote images for all media.")
			downloadImages = false
		}
		s.DownloadImages = downloadImages

		enableTranscoding, err := strconv.ParseBool(os.Getenv("ENABLE_TRANSCODING"))
		if err != nil {
			log.Println("Fail to get ENABLE_TRANSCODING value from environment variables.  Media transcoding will be disabled.")
			enableTranscoding = false
		} else {
			s.EnableHardwareAcceleration = enableTranscoding

			if enableTranscoding {
				s.HardwareAccelerationMethod = os.Getenv("HARDWARE_ACCELERATION_METHOD")
				if s.HardwareAccelerationMethod == "" {
					log.Println("Fail to get HARDWARE_ACCELERATION_METHOD from environment variables.  Defaulting to CPU transcoding.")
					s.HardwareAccelerationMethod = "cpu"
				}
			}
		}

		s.LogsDir = os.Getenv("LOGS_DIR")
		if s.LogsDir == "" {
			log.Println("Fail to get LOGS_DIR value from environment variables.  Logs directory will be placed in the current working directory.")
			s.LogsDir = "logs"
		}

		s.StaticDir = os.Getenv("STATIC_DIR")
		if s.StaticDir == "" {
			log.Println("Fail to get STATIC_DIR value from environment variables.  Static directory will be placed in the current working directory.")
			s.StaticDir = "static"
		}

		s.TranscodeDir = os.Getenv("TRANSCODE_DIR")
		if s.TranscodeDir == "" {
			log.Println("Fail to get TRANSCODE_DIR from environment variables.  Transcoding directory will be placed on the current working directory.")
			s.TranscodeDir = "transcode"
		}

		s.BaseUrl = os.Getenv("BASE_URL")
		if s.BaseUrl == "" {
			s.BaseUrl = "localhost"
		}

		s.FfmpegPath = os.Getenv("FFMPEG_PATH")
		s.FfprobePath = os.Getenv("FFPROBE_PATH")
		s.TmdbApiKey = os.Getenv("TMDB_API_KEY")
		s.JellyfinToken = os.Getenv("JELLYFIN_TOKEN")
		s.MoviesDir = os.Getenv("MOVIES_DIR")
		s.MusicDir = os.Getenv("MUSIC_DIR")
		s.TvshowsDir = os.Getenv("TVSHOWS_DIR")
		s.MoviesImgDir = os.Getenv("MOVIES_IMG_DIR")
		s.StudiosImgDir = os.Getenv("STUDIOS_IMG_DIR")
		s.ArtistsImgDir = os.Getenv("ARTISTS_IMG_DIR")
		s.AvatarImgDir = os.Getenv("AVATAR_IMG_DIR")
		s.PlexToken = os.Getenv("PLEX_TOKEN")
		s.SpotifyClientID = os.Getenv("SPOTIFY_CLIENT_ID")
		s.SpotifyClientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")

		err = app.InitDirs(&s)
		if err != nil {
			return err
		}

		settings, err = app.Queries.CreateSettings(ctx, s)
		if err != nil {
			return err
		}

		if settings.MusicDir != "" {
			go app.ScanMusicLibrary()
		}
	}

	app.Settings = &settings

	return nil
}

func (app *Application) InitDefaultUser(ctx context.Context) error {
	count, err := app.Queries.GetTotalUsersCount(ctx)
	if err != nil {
		return err
	}

	if count == 0 {
		hashPassword, err := helpers.HashPassword(DEFAULT_PASSWORD)
		if err != nil {
			return err
		}

		_, err = app.Queries.CreateUser(ctx, database.CreateUserParams{
			Name:     "Admin User",
			Email:    "admin@example.com",
			Username: "admin",
			Password: hashPassword,
			IsActive: true,
			IsAdmin:  true,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) InitDirs(s *database.CreateSettingsParams) error {
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

func (app *Application) InitRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)

	router.Route("/api/v1", func(r chi.Router) {
		r.Route("/music", func(r chi.Router) {
			r.Get("/musician/count", app.GetMusicianCount)
		})
	})

	return router
}
