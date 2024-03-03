package main

import (
	"igloo/handlers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func movieRoutes(app *fiber.App, db *gorm.DB) {
	h := handlers.NewMovieHandlers(db)

	group := app.Group("/api/v1/movie")

	group.Get("/:id", h.GetMovieByID)
	group.Get("/", h.GetMoviesWithPagination)
	group.Post("", h.CreateMovie)
	group.Put("/", h.UpdateMovie)
}
