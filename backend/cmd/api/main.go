package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/handlers"
	"igloo/cmd/internal/settings"
	"igloo/cmd/internal/tmdb"

	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	settings, err := settings.New()
	if err != nil {
		log.Fatal(err)
	}

	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		settings.PostgresUser,
		settings.PostgresPass,
		settings.PostgresHost,
		settings.PostgresPort,
		settings.PostgresDB,
		settings.PostgresSslMode,
	)

	dbConfig, err := pgxpool.ParseConfig(dbUrl)
	if err != nil {
		log.Fatal(err)
	}

	dbpool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer dbpool.Close()

	err = dbpool.Ping(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	ffmpegBin, err := ffmpeg.New(settings.FfmpegPath)
	if err != nil {
		log.Fatal(err)
	}

	ffprobeBin, err := ffprobe.New(settings.FfprobePath)
	if err != nil {
		log.Fatal(err)
	}

	tmdbClient, err := tmdb.New(&settings.TmdbKey)
	if err != nil {
		log.Fatal(err)
	}

	appHandlers := handlers.New(&handlers.HandlersConfig{
		Ffmpeg:  ffmpegBin,
		Ffprobe: ffprobeBin,
		Tmdb:    tmdbClient,
		Queries: queries,
	})

	f := fiber.New(fiber.Config{
		AppName:               "Igloo API",
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           10 * time.Second,
		DisableStartupMessage: settings.Debug,
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
		EnableStackTrace: settings.Debug,
	}))

	f.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	movies := f.Group("/api/v1/movies")
	movies.Get("/latest", appHandlers.GetLatestMovies)
	movies.Get("/:id", appHandlers.GetMovieByID)

	log.Fatal(f.Listen(":8080"))
}
