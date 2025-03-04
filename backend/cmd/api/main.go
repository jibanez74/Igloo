package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/config"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers" // Alias to avoid conflict
	"igloo/cmd/internal/tmdb"
	"igloo/cmd/internal/tokens"
	"log"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	settings *database.GlobalSetting
	db       *pgxpool.Pool
	queries  *database.Queries
	ffmpeg   ffmpeg.FFmpeg
	ffprobe  ffprobe.Ffprobe
	tmdb     tmdb.Tmdb
	tokens   tokens.TokenManager
}

const DefaultPassword = "AdminPassword"

func main() {
	app, err := initApp()
	if err != nil {
		log.Fatalf("unable to initialize application: %s", err)
	}

	f := fiber.New(fiber.Config{
		AppName:               "Igloo API",
		DisableStartupMessage: false,
	})

	f.Use(recover.New(recover.Config{
		EnableStackTrace: app.settings.Debug,
	}))

	f.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	api := f.Group("/api/v1")

	api.Static("/transcode", app.settings.TranscodeDir, fiber.Static{
		Compress:  true,
		Browse:    false,
		Index:     "",
		ByteRange: true,
		Download:  false,
		Next:      nil,
	})

	api.Static("/static", app.settings.StaticDir, fiber.Static{
		Compress: true,
		Browse:   true,
	})
	auth := api.Group("/auth")
	auth.Post("/login", app.login)

	movies := api.Group("/movies")
	movies.Get("/count", app.getTotalMovieCount)
	movies.Get("/latest", app.getLatestMovies)
	movies.Get("/", app.getMoviesPaginated)
	movies.Post("/create", app.createTmdbMovie)
	movies.Get("/:id", app.getMovieDetails)

	err = f.Listen(fmt.Sprintf(":%d", app.settings.Port))
	if err != nil {
		log.Fatal(err)
	}
}

func initApp() (*application, error) {
	var app application

	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	err = app.initDB()
	if err != nil {
		return nil, err
	}

	var settings database.GlobalSetting

	if cfg.CreateSettings {
		settings, err = app.queries.CreateSettings(context.Background(), *cfg.Settings)
		if err != nil {
			return nil, fmt.Errorf("failed to create settings: %w", err)
		}
	} else {
		settings, err = app.queries.GetSettings(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get settings: %w", err)
		}
	}
	app.settings = &settings

	err = app.createDefaultUser()
	if err != nil {
		return nil, err
	}

	err = app.initTokens()
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

func (app *application) createDefaultUser() error {
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

func (app *application) initTokens() error {
	jwtAccessSecret := os.Getenv("JWT_ACCESS_SECRET")
	if jwtAccessSecret == "" {
		jwtAccessSecret = "your-256-bit-secret"
	}

	jwtRefreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if jwtRefreshSecret == "" {
		jwtRefreshSecret = "your-256-bit-refresh-secret"
	}

	tokenConfig := tokens.DefaultConfig().
		WithAccessTokenSecret(jwtAccessSecret).
		WithRefreshTokenSecret(jwtRefreshSecret)

	tokenManager, err := tokens.New(tokenConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize token manager: %w", err)
	}

	app.tokens = tokenManager

	return nil
}
