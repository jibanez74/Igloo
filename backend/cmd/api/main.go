package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
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
<<<<<<< HEAD
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/jackc/pgx/v5/pgxpool"
)

type config struct {
	db      *database.Queries
	workDir string
	tmdb    tmdb.Tmdb
	ffmpeg  ffmpeg.FFmpeg
	store   *session.Store
}

func main() {
	var app config

	dbpool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("unable to connect to database:", err)
	}
	defer dbpool.Close()

	app.db = database.New(dbpool)
=======
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	settings, err := settings.New()
	if err != nil {
		log.Fatal(err)
	}
>>>>>>> main

	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		settings.PostgresUser,
		settings.PostgresPass,
		settings.PostgresHost,
		settings.PostgresPort,
		settings.PostgresDB,
		settings.PostgresSslMode,
	)

	queries, err := getQueries(&dbUrl)
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

<<<<<<< HEAD
	f.Static("/api/v1/static", app.settings.StaticDir, fiber.Static{
		Compress:      true,
		ByteRange:     true,
		Browse:        false,
		CacheDuration: 24 * time.Hour,
	})

	auth := f.Group("/api/v1/auth")
	auth.Get("/me", app.requireAuth, app.getAuthUser)
	auth.Post("/login", app.login)
	auth.Post("/logout", app.logout)

	ffmpegRoutes := f.Group("/api/v1/ffmpeg")
	ffmpegRoutes.Post("/hls/movie", app.createMovieHls)

=======
>>>>>>> main
	movies := f.Group("/api/v1/movies")
	movies.Get("/latest", appHandlers.GetLatestMovies)
	movies.Get("/:id", appHandlers.GetMovieByID)

	log.Fatal(f.Listen(":8080"))
}

func getQueries(dbUrl *string) (*database.Queries, error) {
	dbConfig, err := pgxpool.ParseConfig(*dbUrl)
	if err != nil {
		return nil, err
	}

	dbpool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}
	defer dbpool.Close()

	err = dbpool.Ping(context.Background())
	if err != nil {
		return nil, err
	}

	return database.New(dbpool), nil
}
