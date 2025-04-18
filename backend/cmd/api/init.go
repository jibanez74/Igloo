package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
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
		f, err := ffmpeg.New(app.settings)
		if err != nil {
			return nil, err
		}

		app.ffmpeg = f
	}

	if app.settings.FfprobePath != "" {
		f, err := ffprobe.New(app.settings)
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

	if count != 0 {
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
			s.Secret = uuid.NewString()
		}

		err = app.initDirs(&s)
		if err != nil {
			return err
		}

		enableTranscoding, err := strconv.ParseBool(os.Getenv("ENABLE_TRANSCODING"))
		if err != nil {
			enableTranscoding = false
		}
		s.EnableHardwareAcceleration = enableTranscoding

		s.HardwareEncoder = os.Getenv("HARDWARE_ENCODER")
		s.FfmpegPath = os.Getenv("FFMPEG_PATH")
		s.FfprobePath = os.Getenv("FFPROBE_PATH")
		s.TmdbApiKey = os.Getenv("TMDB_API_KEY")
		s.JellyfinToken = os.Getenv("JELLYFIN_TOKEN")
		s.MoviesDirList = os.Getenv("MOVIES_DIR")

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

	sharePath := filepath.Join(homeDir, ".local", "share", "igloo")
	err = helpers.CreateDir(sharePath)
	if err != nil {
		return err
	}

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

	// Preserve existing directory list settings if they exist
	if s.MoviesDirList == "" {
		s.MoviesDirList = os.Getenv("MOVIES_DIR_LIST")
	}
	if s.MusicDirList == "" {
		s.MusicDirList = os.Getenv("MUSIC_DIR_LIST")
	}
	if s.TvshowsDirList == "" {
		s.TvshowsDirList = os.Getenv("TVSHOWS_DIR_LIST")
	}

	s.TranscodeDir = transcodeDir
	s.StaticDir = staticDir
	s.MoviesImgDir = moviesImgDir
	s.StudiosImgDir = studiosImgDir
	s.ArtistsImgDir = artistsImgDir
	s.AvatarImgDir = avatarImgDir

	return nil
}
