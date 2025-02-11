package main

import "github.com/gofiber/fiber/v2"

func (app *application) getLatestMovies(c *fiber.Ctx) error {
	movies, err := app.queries.GetLatestMovies(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movies": movies,
	})
}
