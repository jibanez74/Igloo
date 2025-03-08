package main

import (
	"igloo/cmd/internal/tokens"

	"github.com/gofiber/fiber/v2"
)

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (app *application) checkRefreshToken(c *fiber.Ctx) error {
	var req refreshRequest

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Refresh token is required",
		})
	}

	claims, err := app.tokens.ValidateToken(req.RefreshToken, tokens.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid refresh token",
		})
	}

	c.Locals("user_id", int32(claims.UserID))

	return c.Next()
}
