package main

import (
	"github.com/gofiber/fiber/v2"
)

// getNowPlayingMovies fetches movies currently playing in theaters from TMDB
func (app *config) getNowPlayingMovies(c *fiber.Ctx) error {
	movies, err := app.tmdb.GetNowPlayingMovies()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movies": movies.Results,
	})
}
