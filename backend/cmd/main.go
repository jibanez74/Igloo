package main

import (
	"igloo/cmd/database"
	"igloo/cmd/database/models"
	"igloo/cmd/repository"
	"igloo/cmd/tmdb"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/redis"
)

type config struct {
	Repo         repository.Repo
	Settings     *models.GlobalSettings
	workDir      string
	Tmdb         tmdb.Tmdb
	Session      *session.Store
	CookieDomain string
	CookieSecret string
}

func main() {
	var app config

	db, err := database.New()
	if err != nil {
		panic(err)
	}
	app.Repo = repository.New(db)

	app.Settings, err = app.Repo.GetSettings()
	if err != nil {
		panic(err)
	}

	app.workDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	cookieSecret := os.Getenv("COOKIE_SECRET")
	if cookieSecret == "" {
		panic("COOKIE_SECRET is not set")
	}
	app.CookieSecret = cookieSecret

	app.CookieDomain = os.Getenv("COOKIE_DOMAIN")
	if app.CookieDomain == "" {
		panic("COOKIE_DOMAIN is not set")
	}

	app.Tmdb, err = tmdb.New(&app.Settings.TmdbKey)
	if err != nil {
		log.Println("tmdb api key not set")
	}

	err = app.run()
	if err != nil {
		panic(err)
	}
}

func (app *config) run() error {
	f := fiber.New(fiber.Config{
		AppName:               "Igloo API",
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           10 * time.Second,
		DisableStartupMessage: app.Settings.Debug,
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
		EnableStackTrace: app.Settings.Debug,
	}))

	f.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

	f.Static("/api/v1/static", app.Settings.StaticDir, fiber.Static{
		Compress:      true,
		ByteRange:     true,
		Browse:        false,
		CacheDuration: 24 * time.Hour,
	})

	auth := f.Group("/api/v1/auth")
	auth.Post("/login", app.Login)
	auth.Get("/logout", app.Logout)

	movies := f.Group("/api/v1/movies")
	movies.Get("/all", app.GetAllMovies)
	movies.Get("/latest", app.GetLatestMovies)
	movies.Get("/:id", app.GetMovieByID)
	movies.Post("/create", app.CreateMovie)

	users := f.Group("/api/v1/users")
	users.Get("/me", app.IsAuth, app.GetAuthUser)

	if !app.Settings.Debug {
		clientDir := filepath.Join(app.workDir, "cmd", "client")
		assetsDir := filepath.Join(clientDir, "assets")

		f.Static("/assets", assetsDir, fiber.Static{
			Compress:      true,
			ByteRange:     true,
			Browse:        false,
			CacheDuration: 24 * time.Hour,
		})

		f.Get("*", func(c *fiber.Ctx) error {
			indexFile := filepath.Join(clientDir, "index.html")

			_, err := os.Stat(indexFile)
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Page not found",
				})
			}

			c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Set("Pragma", "no-cache")
			c.Set("Expires", "0")

			return c.SendFile(indexFile)
		})
	}

	return f.Listen(":8080")
}

func (app *config) initSession() {
	storage := redis.New(redis.Config{
		Host:      app.Settings.RedisHost,
		Port:      app.Settings.RedisPort,
		Username:  app.Settings.RedisUser,
		Password:  app.Settings.RedisPassword,
		Database:  0,
		Reset:     false,
		PoolSize:  10,
		TLSConfig: nil,
	})

	app.Session = session.New(session.Config{
		Storage:        storage,
		Expiration:     24 * time.Hour,
		KeyLookup:      "cookie:session_id",
		CookieDomain:   app.CookieDomain,
		CookiePath:     "/",
		CookieSecure:   !app.Settings.Debug,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
	})
}
