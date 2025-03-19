package main

import (
	"igloo/cmd/internal/helpers"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (app *application) validateTokenInHeader(c *fiber.Ctx) error {
	accessToken := c.Get("Authorization")
	if accessToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": notAuthMsg})
	}

	parts := strings.Split(accessToken, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": notAuthMsg})
	}

	token := parts[1]

	err := helpers.VerifyAccessToken(token, app.settings)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": notAuthMsg})
	}

	return c.Next()
}
