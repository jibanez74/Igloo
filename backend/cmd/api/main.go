package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/tmdb"
	"log"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/redis/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	db       *pgxpool.Pool
	queries  *database.Queries
	redis    *redis.Storage
	session  *session.Store
	settings *database.GlobalSetting
	ffmpeg   ffmpeg.FFmpeg
	ffprobe  ffprobe.Ffprobe
	tmdb     tmdb.Tmdb
}

const (
	DefaultPassword = "AdminPassword"
	authErr         = "invalid credentials"
	serverErr       = "server error"
	notAuthMsg      = "not authorized"
)

func main() {
	app, err := initApp()
	if err != nil {
		log.Fatalf("unable to initialize application: %s", err)
	}

	f := fiber.New(fiber.Config{
		AppName:               "Igloo API",
		Prefork:               true,
		DisableStartupMessage: !app.settings.Debug,
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
		Index:    "",
		Next:     nil,
	})

	auth := api.Group("/auth")
	auth.Get("/me", app.validateSession, app.getAuthUser)
	auth.Post("/login", app.login)
	auth.Post("/logout", app.logout)

	movies := api.Group("/movies")
	movies.Get("/count", app.getTotalMovieCount)
	movies.Get("/latest", app.getLatestMovies)
	movies.Get("/", app.getMoviesPaginated)
	movies.Post("/create", app.createTmdbMovie)
	movies.Get("/:id", app.getMovieDetails)

	users := api.Group("/users")
	users.Get("/", app.getUsersPaginated)
	users.Post("/create", app.createUser)

	if app.settings.Debug {
		workDir, err := os.Getwd()
		if err != nil {
			log.Fatal(fmt.Errorf("unable to get working directory: %w", err))
		}

		clientDir := filepath.Join(workDir, "cmd", "client")

		f.Static("/assets", filepath.Join(clientDir, "assets"), fiber.Static{
			Compress: true,
			Browse:   false,
			Index:    "",
			Next:     nil,
		})

		f.Get("*", func(c *fiber.Ctx) error {
			file, err = os.Stat(clientDir, "index.html")
			if err != nil {
				log.Fatal(fmt.Errorf("unable to get index.html file: %w", err))
			}

			return c.SendFile(file)
		})

	}

	log.Fatal(f.Listen(fmt.Sprintf(":%d", app.settings.Port)))
}
