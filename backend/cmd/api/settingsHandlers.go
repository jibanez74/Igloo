package main

import (
	"igloo/cmd/internal/database"

	"github.com/gofiber/fiber/v2"
)

func (app *application) getSettings(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"settings": app.settings,
	})
}

func (app *application) updateSettings(c *fiber.Ctx) error {
	var req database.UpdateSettingsParams

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse request for updating settings",
		})
	}

	settings, err := app.queries.UpdateSettings(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to update settings",
		})
	}

	app.settings = &settings

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"settings": app.settings,
	})
}
