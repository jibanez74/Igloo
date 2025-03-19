package main

import (
	"igloo/cmd/internal/helpers"

	"github.com/gofiber/fiber/v2"
)

func (app *application) validateTokenInHeader(c *fiber.Ctx) error {
	token := c.Cookies("access_token")
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": notAuthMsg})
	}

	err := helpers.VerifyAccessToken(token, app.settings)
	if err != nil {
		c.ClearCookie("access_token")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": notAuthMsg})
	}

	return c.Next()
}
