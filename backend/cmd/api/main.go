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

	"github.com/fsnotify/fsnotify"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	db           *pgxpool.Pool
	queries      *database.Queries
	settings     *database.GlobalSetting
	ffmpeg       ffmpeg.FFmpeg
	ffprobe      ffprobe.Ffprobe
	tmdb         tmdb.Tmdb
	movieWatcher *fsnotify.Watcher
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

	f.Use(cors.New())

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

	if app.settings.MoviesDirList != "" {
		api.Static("/media/movies", app.settings.MoviesDirList, fiber.Static{
			Compress:  true,
			Browse:    false,
			Download:  true,
			ByteRange: true,
			Next:      nil,
		})
	}

	auth := api.Group("/auth")
	auth.Get("/me", app.validateTokenInHeader, app.getCurrentUser)
	auth.Post("/login", app.login)
	auth.Post("/logout", app.validateTokenInHeader, app.logout)

	movies := api.Group("/movies")
	movies.Get("/count", app.getTotalMovieCount)
	movies.Get("/latest", app.getLatestMovies)
	movies.Get("/", app.getMoviesPaginated)
	movies.Post("/create", app.createTmdbMovie)
	movies.Get("/:id/details", app.getMovieDetails)
	movies.Get("/:id/direct-play", app.getMovieForDirectPlayback)

	settings := api.Group("/settings")
	settings.Get("/", app.getSettings)
	settings.Put("/", app.updateSettings)

	users := api.Group("/users")
	users.Get("/", app.getUsersPaginated)
	users.Get("/:id", app.getUserByID)
	users.Post("/create", app.createUser)
	users.Patch("/:id/avatar", app.updateUserAvatar)
	users.Delete("/delete/:id", app.deleteUser)

	if !app.settings.Debug {
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

		f.Static("*", clientDir, fiber.Static{
			Compress: true,
			Browse:   false,
			Index:    "index.html",
			Next:     nil,
		})
	}

	log.Fatal(f.Listen(fmt.Sprintf(":%d", app.settings.Port)))
}
