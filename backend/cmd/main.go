package main

import (
	"errors"
	"igloo/cmd/database"
	"igloo/cmd/repository"
	"igloo/cmd/session"
	"igloo/cmd/tmdb"
	"strconv"

	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	ses "github.com/gofiber/fiber/v2/middleware/session"
)

type config struct {
	repo    repository.Repo
	session *ses.Store
	tmdb    tmdb.Tmdb
}

func main() {
	var app config

	db, err := database.New()
	if err != nil {
		panic(err)
	}
	app.repo = repository.New(db)

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	nport, err := strconv.Atoi(redisPort)
	if err != nil {
		panic(err)
	}
	app.session = session.New(nport)

	tmdbKey := os.Getenv("TMDB_API_KEY")
	if tmdbKey == "" {
		panic(errors.New("TMDB_API_KEY is not set"))
	}
	app.tmdb = tmdb.New(tmdbKey)

	// go app.listenForShutDown()

	serverPort := os.Getenv("PORT")
	if serverPort == "" {
		serverPort = ":8080"
	}

	err = app.runServer(serverPort)
	if err != nil {
		panic(err)
	}
}

func (app *config) runServer(p string) error {
	api := fiber.New()
	api.Use(recover.New())
	api.Use(logger.New())
	api.Use(healthcheck.New())

	movieRouter := api.Group("/api/v1/movie")
	movieRouter.Get("/:id", app.GetMovieByID)
	movieRouter.Get("/api/v1/movie/latest", app.GetLatestMovies)
	movieRouter.Get("", app.GetMoviesWithPagination)
	movieRouter.Post("", app.CreateMovie)

	streamingRouter := api.Group("/api/v1/stream")
	streamingRouter.Get("/video/:id", app.DirectStreamVideo)

	return api.Listen(p)
}

// func (app *config) listenForShutDown() {
// }
