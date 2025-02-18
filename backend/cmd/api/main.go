package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	customLogger "igloo/cmd/internal/logger" // Alias to avoid conflict
	"igloo/cmd/internal/session"
	"igloo/cmd/internal/session/caching"
	"igloo/cmd/internal/settings"
	"igloo/cmd/internal/tmdb"
	"log"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v5/pgxpool"
)

type imageJob struct {
	sourceURL  string
	targetPath string
	filename   string
	onSuccess  func(string)
	onError    func(error)
}

type application struct {
	settings   settings.Settings
	caching    *redis.Pool
	session    session.Session
	db         *pgxpool.Pool
	queries    *database.Queries
	ffmpeg     ffmpeg.FFmpeg
	ffprobe    ffprobe.Ffprobe
	tmdb       tmdb.Tmdb
	logger     customLogger.AppLogger
	imageQueue chan imageJob
}

const (
	DefaultPassword = "AdminPassword"
	DefaultImageJobs = 10
	ServerTimeout   = 10 * time.Second
	ShutdownTimeout = 10 * time.Second
)

func main() {
	app, err := initApp()
	if err != nil {
		log.Fatalf("unable to initialize application: %s", err)
	}

	f := fiber.New(fiber.Config{
		AppName:               "Igloo API",
		ReadTimeout:           ServerTimeout,
		WriteTimeout:          ServerTimeout,
		IdleTimeout:           ServerTimeout,
		DisableStartupMessage: false,
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
		EnableStackTrace: app.settings.GetDebug(),
	}))

	f.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}))

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
	movies.Post("/create-hls/:id", app.createMovieHlsStream)
	movies.Get("/stream/:id", app.streamMovie)
	movies.Get("/:id", app.getMovieDetails)

	users := api.Group("/users")
	users.Get("/", app.getUsersPaginated)
	users.Get("/count", app.getTotalUsersCount)
	users.Get("/:id", app.getUserByID)
	users.Post("/create", app.createUser)

	serverChan := make(chan error, 1)

	go func() {
		err = f.Listen(app.settings.GetPort())
		if err != nil {
			serverChan <- err
		}
	}()

	select {
	case err := <-serverChan:
		app.logger.Fatal(fmt.Sprintf("error starting server: %s", err))

	case <-time.After(5 * time.Second):
		app.logger.Info(fmt.Sprintf("Server started successfully on port %s", app.settings.GetPort()))
		app.listenForShutdown(f)
	}
}

func initApp() (*application, error) {
	settings, err := settings.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	logger, err := customLogger.New(settings.GetDebug(), settings.GetStaticDir())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
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
		return nil, errors.New(fmt.Sprintf("unable to check if there are any users in the database: %s", err))
	}
	if count == 0 {
		hashPassword, err := helpers.HashPassword(DefaultPassword)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("unable to hash password for default user creation: %s", err))
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
			return nil, errors.New(fmt.Sprintf("unable to create default user: %s", err))
		}
	}

	redisPool := caching.New(settings.GetRedisAddress())

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

	app := &application{
		settings:   settings,
		caching:    redisPool,
		session:    sessionManager,
		db:         dbpool,
		queries:    queries,
		ffmpeg:     ffmpegBin,
		ffprobe:    ffprobeBin,
		tmdb:       tmdbClient,
		logger:     logger,
		imageQueue: make(chan imageJob, DefaultImageJobs),
	}

	// Start image processing workers
	go app.processImages()

	return app, nil
}

func (app *application) processImages() {
	for job := range app.imageQueue {
		fullPath, err := helpers.SaveImage(
			job.sourceURL,
			job.targetPath,
			job.filename,
		)

		if err != nil {
			app.logger.Error(fmt.Errorf("failed to save image %s: %w", job.sourceURL, err))
			if job.onError != nil {
				job.onError(err)
			}
			continue
		}

		if job.onSuccess != nil {
			job.onSuccess(*fullPath)
		}
	}
}

func (app *application) listenForShutdown(server *fiber.App) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	app.logger.Info("Starting graceful shutdown...")

	// Create a timeout context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	// 1. First shutdown server to stop accepting new requests
	if err := server.ShutdownWithContext(ctx); err != nil {
		app.logger.Error(fmt.Errorf("error shutting down server: %w", err))
	}

	// 2. Close image processing to stop accepting new jobs
	close(app.imageQueue)
	app.logger.Info("Closed image processing queue")

	// 3. Close Redis connections
	if err := app.caching.Close(); err != nil {
		app.logger.Error(fmt.Errorf("error closing redis connection pool: %w", err))
	} else {
		app.logger.Info("Closed Redis connection pool")
	}

	// 4. Close database connections
	app.db.Close()
	app.logger.Info("Closed database connections")

	// 5. Finally close logger
	if err := app.logger.Close(); err != nil {
		// Can't use logger here since it's closed
		log.Printf("error closing logger: %v", err)
	}
}
