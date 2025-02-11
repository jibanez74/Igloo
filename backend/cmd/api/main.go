package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/settings"
	"igloo/cmd/internal/tmdb"

	"log"
	"time"

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
}

func main() {
	app, err := initApp()
	if err != nil {
		msg := fmt.Sprintf("unable to initialize application: %s", err)
		log.Fatal(msg)
	}
	defer app.db.Close()

	f := fiber.New(fiber.Config{
		AppName:               "Igloo API",
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           10 * time.Second,
		DisableStartupMessage: app.settings.GetDebug(),
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError

			e, ok := err.(*fiber.Error)
			if ok {
				code = e.Code
			}

			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	f.Use(recover.New(recover.Config{
		EnableStackTrace: app.settings.GetDebug(),
	}))

	f.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	movies := f.Group("/api/v1/movies")
	movies.Get("/latest", app.getLatestMovies)
	movies.Post("/create", app.createTmdbMovie)

	log.Fatal(f.Listen(app.settings.GetPort()))
}

func initApp() (*application, error) {
	settings, err := settings.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
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

	return &application{
		settings: settings,
		db:       dbpool,
		queries:  queries,
		ffmpeg:   ffmpegBin,
		ffprobe:  ffprobeBin,
		tmdb:     tmdbClient,
	}, nil
}
