package main

import (
	"context"
	"fmt"
	"igloo/handlers"
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.uber.org/fx"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const PORT = ":8000"

func startUpServer(lc fx.Lifecycle, appHandlers *handlers.AppHandlers) *fiber.App {
	api := fiber.New()
	api.Use(recover.New())
	api.Use(logger.New())
	api.Use(healthcheck.New())

	api.Static("/api/v1/public", "./public")

	homeRoutes := api.Group("/api/v1/recent")
	homeRoutes.Get("", appHandlers.GetRecent)

	artistRoutes := api.Group("/api/v1/artist")
	artistRoutes.Get("/:id", appHandlers.GetArtistByID)
	artistRoutes.Post("", appHandlers.FindOrCreateArtist)

	genreRoutes := api.Group("/api/v1/genre")
	genreRoutes.Post("", appHandlers.FindOrCreateGenre)

	studioRoutes := api.Group("/api/v1/studio")
	studioRoutes.Get("/:id", appHandlers.GetStudioByID)
	studioRoutes.Post("", appHandlers.CreateStudio)

	movieRoutes := api.Group("/api/v1/movie")
	movieRoutes.Get("/direct-play", appHandlers.DirectPlayMovie)
	movieRoutes.Get("/count", appHandlers.GetMovieCount)
	movieRoutes.Get("/:id", appHandlers.GetMovieByID)
	movieRoutes.Get("", appHandlers.GetMoviesWithPagination)
	movieRoutes.Post("", appHandlers.CreateMovie)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			fmt.Printf("Server started on port %s\n", PORT)
			go api.Listen(PORT)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return api.Shutdown()
		},
	})

	return api
}

func connectToDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("igloo.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(
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
	store := session.New()

	return store
}

func main() {
	fx.New(
		fx.Provide(
			connectToDB,
			initSessions,
			handlers.New,
		),
		fx.Invoke(startUpServer),
	).Run()
}
