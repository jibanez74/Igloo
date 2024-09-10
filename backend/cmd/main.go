package main

import (
	"context"
	"fmt"
	"igloo/cmd/handlers"
	"igloo/cmd/models"
	"igloo/cmd/repository"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/redis"
	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func startUpServer(lc fx.Lifecycle, appHandlers *handlers.Handlers) *fiber.App {
	api := fiber.New()
	api.Use(recover.New())
	api.Use(logger.New())
	api.Use(healthcheck.New())
	api.Use(cors.New())

	artistRoutes := api.Group("/api/v1/artist")
	artistRoutes.Post("", appHandlers.FindOrCreateArtist)

	movieRoutes := api.Group("/api/v1/movie")
	movieRoutes.Get("/count", appHandlers.GetMovieCount)
	movieRoutes.Get("/latest", appHandlers.GetLatestMovies)
	movieRoutes.Get("/:id", appHandlers.GetMovieByID)
	movieRoutes.Post("", appHandlers.CreateMovie)

	playRoutes := api.Group("/api/v1/play")
	playRoutes.Get("/video", appHandlers.PlayVideo)

	transcodeRoutes := api.Group("/api/v1/transcode")
	transcodeRoutes.Get("/cancel/:pid", appHandlers.CancelFfmpegProcess)
	transcodeRoutes.Post("/video", appHandlers.TranscodeVideo)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			fmt.Printf("Server started on port %s\n", os.Getenv("PORT"))
			go api.Listen(os.Getenv("PORT"))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return api.Shutdown()
		},
	})

	return api
}

func connectToDB() *gorm.DB {
	dsn := os.Getenv("DSN")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(
		&models.User{},
		&models.Movie{},
		&models.Artist{},
		&models.Chapter{},
		&models.Trailer{},
		&models.Genre{},
		&models.Studio{},
		&models.Cast{},
		&models.Crew{},
		&models.Video{},
		&models.Audio{},
		&models.Subtitles{},
	)

	return db
}

func initSessions() *session.Store {
	redisPort, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if err != nil {
		panic(err)
	}

	redisDb, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		panic(err)
	}

	storage := redis.New(redis.Config{
		Host:     os.Getenv("REDIS_HOST"),
		Port:     redisPort,
		Password: os.Getenv("REDIS_PASSWORD"),
		Database: redisDb,
	})

	store := session.New(session.Config{
		Storage:        storage,
		CookiePath:     os.Getenv("COOKIE_PATH"),
		CookieSecure:   true,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieName:     os.Getenv("COOKIE_NAME"),
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),
		Expiration:     24 * time.Hour,
	})

	return store
}

func main() {
	fx.New(
		fx.Provide(
			connectToDB,
			initSessions,
			repository.New,
			handlers.New,
		),
		fx.Invoke(startUpServer),
	).Run()
}
