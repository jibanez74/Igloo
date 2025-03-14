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
	auth.Get("/me", app.validateTokenInHeader, app.getAuthUser)
	auth.Post("/login", app.login)
	auth.Post("/logout", app.logout)

	movies := api.Group("/movies")
	movies.Get("/count", app.getTotalMovieCount)
	movies.Get("/latest", app.getLatestMovies)
	movies.Get("/", app.getMoviesPaginated)
	movies.Post("/create", app.createTmdbMovie)
	movies.Get("/:id", app.getMovieDetails)
	movies.Post("/create-hls/:id", app.createHlsStream)

	users := api.Group("/users")
	users.Get("/", app.getUsersPaginated)
	users.Post("/create", app.createUser)

	if app.settings.Debug {
		workDir, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		api.Static("/assets", filepath.Join(workDir, "cmd", "client", "dist"), fiber.Static{
			Compress: true,
			Browse:   false,
			Index:    "",
			Next:     nil,
		})

		api.Static("*", filepath.Join(workDir, "cmd", "client"), fiber.Static{
			Compress: true,
			Browse:   false,
			Index:    "index.html",
			Next:     nil,
		})
	}

	log.Fatal(f.Listen(fmt.Sprintf(":%d", app.settings.Port)))
}
