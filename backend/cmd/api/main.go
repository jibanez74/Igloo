package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/session"
	"igloo/cmd/internal/session/caching"
	"igloo/cmd/internal/settings"
	"igloo/cmd/internal/tmdb"

	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5/pgxpool"
)

type application struct {
	settings settings.Settings
	caching  *redis.Pool
	session  session.Session
	db       *pgxpool.Pool
	queries  *database.Queries
	ffmpeg   ffmpeg.FFmpeg
	ffprobe  ffprobe.Ffprobe
	tmdb     tmdb.Tmdb
}

const DefaultPassword = "AdminPassword"

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

	f.Use(logger.New())

	api := f.Group("/api/v1")

	auth := api.Group("/auth")
	auth.Post("/login", app.login)
	auth.Get("/me", app.getAuthUser)
	auth.Post("/logout", app.logout)

	movies := api.Group("/movies")
	movies.Get("/count", app.getTotalMovieCount)
	movies.Get("/latest", app.getLatestMovies)
	movies.Get("/", app.getMoviesPaginated)
	movies.Post("/create", app.createTmdbMovie)
	movies.Get("/stream/:id", app.streamMovie)
	movies.Get("/:id", app.getMovieDetails)

	users := api.Group("/users")
	users.Get("/", app.getUsersPaginated)
	users.Get("/count", app.getTotalUsersCount)
	users.Get("/:id", app.getUserByID)
	users.Post("/create", app.createUser)

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

	count, err := queries.GetTotalUsersCount(context.Background())
	if err != nil {
		msg := fmt.Sprintf("unable to check if there are any users in the database: %s", err)
		log.Println(msg)
	} else {
		if count == 0 {
			hashPassword, err := helpers.HashPassword(DefaultPassword)
			if err != nil {
				msg := fmt.Sprintf("unable to hash password for default user creation: %s", err)
				log.Fatal(msg)
			}

			_, err = queries.CreateUser(context.Background(), database.CreateUserParams{
				Name:     "Admin User",
				Email:    "admin@example.com",
				Username: "admin",
				Password: hashPassword,
				IsActive: true,
				IsAdmin:  true,
			})

			if err != nil {
				msg := fmt.Sprintf("unable to create default user: %s", err)
				log.Fatal(msg)
			}
		}
	}

	redisPool := caching.New(settings.GetRedisAddress())

	// Initialize session with proper configuration
	sessionManager := session.New(
		!settings.GetDebug(),
		redisPool,
	)

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
		caching:  redisPool,
		session:  sessionManager,
		db:       dbpool,
		queries:  queries,
		ffmpeg:   ffmpegBin,
		ffprobe:  ffprobeBin,
		tmdb:     tmdbClient,
	}, nil
}
