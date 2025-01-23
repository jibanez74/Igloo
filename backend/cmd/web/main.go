package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/database/models"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/tmdb"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"gorm.io/gorm"
)

type config struct {
	db       *gorm.DB
	settings *models.GlobalSettings
	workDir  string
	tmdb     tmdb.Tmdb
	ffmpeg   ffmpeg.FFmpeg
}

func main() {
	var app config

	db, err := database.New()
	if err != nil {
		panic(err)
	}
	app.db = db

	err = db.First(app.settings).Error
	if err != nil {
		panic(err)
	}

	app.ffmpeg, err = ffmpeg.New(app.settings.Ffmpeg)
	if err != nil {
		panic(err)
	}

	app.workDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	app.tmdb, err = tmdb.New(&app.settings.TmdbKey)
	if err != nil {
		panic(err)
	}

	f := fiber.New(fiber.Config{
		AppName:               "Igloo API",
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           10 * time.Second,
		DisableStartupMessage: app.settings.Debug,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError

			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	f.Use(recover.New(recover.Config{
		EnableStackTrace: app.settings.Debug,
	}))

	f.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	f.Static("/api/v1/static", app.settings.StaticDir, fiber.Static{
		Compress:      true,
		ByteRange:     true,
		Browse:        false,
		CacheDuration: 24 * time.Hour,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	movies := f.Group("/api/v1/movies")
	movies.Get("/latest", app.getLatestMovies)
	movies.Post("/create", app.createMovie)

	serverShutdown := make(chan struct{})

	go func() {
		if err := f.Listen(port); err != nil {
			log.Printf("Fatal server error: %v\n", err)
			close(serverShutdown)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Gracefully shutting down...")

		err = f.Shutdown()
		if err != nil {
			log.Printf("Error shutting down server: %v\n", err)
		}

	case <-serverShutdown:
		log.Println("Server was shut down unexpectedly")
	}

	sqlDB, err := app.db.DB()
	if err == nil {
		err = sqlDB.Close()
		if err != nil {
			log.Printf("Error closing database connection: %v\n", err)
		} else {
			log.Println("Database connection closed")
		}
	}

	log.Println("Server exited")
}
