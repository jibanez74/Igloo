package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/caching"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

func initApp() (*application, error) {
	var app application

	err := app.initDB()
	if err != nil {
		return nil, err
	}

	err = app.initSettings()
	if err != nil {
		return nil, err
	}

	err = app.initDefaultUser()
	if err != nil {
		return nil, err
	}

	if app.settings.FfmpegPath != "" {
		f, err := ffmpeg.New(app.settings.FfmpegPath)
		if err != nil {
			return nil, err
		}

		app.ffmpeg = f
	}

	if app.settings.FfprobePath != "" {
		f, err := ffprobe.New(app.settings.FfprobePath)
		if err != nil {
			return nil, err
		}

		app.ffprobe = f
	}

	if app.settings.TmdbApiKey != "" {
		t, err := tmdb.New(&app.settings.TmdbApiKey)
		if err != nil {
			return nil, err
		}

		app.tmdb = t
	}

	app.redis = caching.New()

	store := session.New(session.Config{
		Storage:        app.redis,
		Expiration:     time.Hour * 24 * 30,
		CookieDomain:   app.settings.CookieDomain,
		CookiePath:     app.settings.CookiePath,
		CookieSecure:   !app.settings.Debug,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
	})

	app.session = store

	return &app, nil
}

func (app *application) initDB() error {
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

	app.db = dbpool
	app.queries = queries

	return nil
}

func (app *application) initSettings() error {
	count, err := app.queries.GetSettingsCount(context.Background())
	if err != nil {
		return fmt.Errorf("unable to determine settings count: %w", err)
	}

	var settings database.GlobalSetting

	if count == 0 {
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

		s.BaseUrl = os.Getenv("BASE_URL")
		if s.BaseUrl == "" {
			s.BaseUrl = "localhost"
		}

		downloadImages, err := strconv.ParseBool(os.Getenv("DOWNLOAD_IMAGES"))
		if err != nil {
			downloadImages = false
		}
		s.DownloadImages = downloadImages

		s.Audience = os.Getenv("AUDIENCE")
		if s.Audience == "" {
			s.Audience = "igloo"
		}

		s.CookieDomain = os.Getenv("COOKIE_DOMAIN")
		if s.CookieDomain == "" {
			s.CookieDomain = "localhost"
		}

		s.CookiePath = os.Getenv("COOKIE_PATH")
		if s.CookiePath == "" {
			s.CookiePath = "/"
		}

		s.Issuer = os.Getenv("ISSUER")
		if s.Issuer == "" {
			s.Issuer = "igloo"
		}

		s.Secret = os.Getenv("SECRET")
		if s.Secret == "" {
			s.Secret = fmt.Sprintf("secret_igloo_%d", time.Now().UnixNano())
		}

		err = app.initDirs(&s)
		if err != nil {
			return err
		}

		s.FfmpegPath = os.Getenv("FFMPEG_PATH")
		s.HardwareAcceleration = os.Getenv("HARDWARE_ACCELERATION")
		s.FfprobePath = os.Getenv("FFPROBE_PATH")
		s.TmdbApiKey = os.Getenv("TMDB_API_KEY")
		s.JellyfinToken = os.Getenv("JELLYFIN_TOKEN")

		settings, err = app.queries.CreateSettings(context.Background(), s)
		if err != nil {
			return fmt.Errorf("failed to create settings: %w", err)
		}
	} else {
		settings, err = app.queries.GetSettings(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get settings: %w", err)
		}
	}

	app.settings = &settings

	return nil
}

func (app *application) initDefaultUser() error {
	count, err := app.queries.GetTotalUsersCount(context.Background())
	if err != nil {
		return fmt.Errorf("unable to determine total users count: %w", err)
	}

	if count == 0 {
		hashPassword, err := helpers.HashPassword(DefaultPassword)
		if err != nil {
			return fmt.Errorf("failed to hash password for default user: %w", err)
		}

		_, err = app.queries.CreateUser(context.Background(), database.CreateUserParams{
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

func (app *application) initDirs(s *database.CreateSettingsParams) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get users home directory: %w", err)
	}

	sharePath := filepath.Join(homeDir, ".local", "share")

	transcodeDir := filepath.Join(sharePath, "transcode")
	err = helpers.CreateDir(transcodeDir)
	if err != nil {
		return err
	}

	staticDir := filepath.Join(sharePath, "static")
	err = helpers.CreateDir(staticDir)
	if err != nil {
		return err
	}

	imgDir := filepath.Join(staticDir, "images")

	s.TranscodeDir = transcodeDir
	s.StaticDir = staticDir
	s.MoviesDirList = os.Getenv("MOVIES_DIR_LIST")
	s.MoviesImgDir = filepath.Join(imgDir, "movies")
	s.MusicDirList = os.Getenv("MUSIC_DIR_LIST")
	s.TvshowsDirList = os.Getenv("TVSHOWS_DIR_LIST")
	s.StudiosImgDir = filepath.Join(imgDir, "studios")
	s.ArtistsImgDir = filepath.Join(imgDir, "artists")

	return nil
}
