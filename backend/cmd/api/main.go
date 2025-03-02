package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	customLogger "igloo/cmd/internal/logger" // Alias to avoid conflict
	"igloo/cmd/internal/settings"
	"igloo/cmd/internal/tmdb"
	"igloo/cmd/internal/tokens"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	settings settings.Settings
	db       *pgxpool.Pool
	queries  *database.Queries
	ffmpeg   ffmpeg.FFmpeg
	ffprobe  ffprobe.Ffprobe
	tmdb     tmdb.Tmdb
	logger   customLogger.AppLogger
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
		EnableStackTrace: app.settings.GetDebug(),
	}))

	f.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	api := f.Group("/api/v1")

	auth := api.Group("/auth")
	auth.Post("/login", app.login)

	// Movie routes
	movies := api.Group("/movies")
	movies.Get("/count", app.getTotalMovieCount)
	movies.Get("/latest", app.getLatestMovies)
	movies.Get("/", app.getMoviesPaginated)
	movies.Post("/create", app.createTmdbMovie)
	movies.Get("/:id", app.getMovieDetails)

	err = f.Listen(app.settings.GetPort())
	if err != nil {
		log.Fatal(err)
	}
}

func initApp() (*application, error) {
	settings, err := settings.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	logger, err := customLogger.New(settings.GetDebug(), settings.GetStaticDir())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		settings.GetPostgresUser(),
		settings.GetPostgresPass(),
		settings.GetPostgresHost(),
		settings.GetPostgresPort(),
		settings.GetPostgresDB(),
		settings.GetPostgresSslMode(),
	)

	dbConfig, err := pgxpool.ParseConfig(dbUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	dbpool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	err = dbpool.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	queries := database.New(dbpool)

	count, err := queries.GetTotalUsersCount(context.Background())
	if err != nil {
		return nil, err
	}
	if count == 0 {
		hashPassword, err := helpers.HashPassword(DefaultPassword)
		if err != nil {
			return nil, err
		}

		_, err = queries.CreateUser(context.Background(), database.CreateUserParams{
			Name:     "Admin User",
			Email:    "admin@example.com",
			Username: "admin",
			Password: hashPassword,
			IsActive: true,
			IsAdmin:  true,
		})

		if err != nil {
			return nil, err
		}
	}

	ffmpegBin, err := ffmpeg.New(settings.GetFfmpegPath())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffmpeg: %w", err)
	}

	ffprobeBin, err := ffprobe.New(settings.GetFfprobePath())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffprobe: %w", err)
	}

	tmdbKey := settings.GetTmdbKey()
	tmdbClient, err := tmdb.New(&tmdbKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tmdb client: %w", err)
	}

	// Initialize token manager
	tokenConfig := tokens.DefaultConfig().
		WithAccessTokenSecret(settings.GetJwtAccessSecret()).
		WithRefreshTokenSecret(settings.GetJwtRefreshSecret())

	tokenManager, err := tokens.New(tokenConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token manager: %w", err)
	}

	app := &application{
		settings: settings,
		db:       dbpool,
		queries:  queries,
		ffmpeg:   ffmpegBin,
		ffprobe:  ffprobeBin,
		tmdb:     tmdbClient,
		logger:   logger,
		tokens:   tokenManager,
	}

	return app, nil
}
