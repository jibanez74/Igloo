package main

import (
	"github.com/gofiber/fiber/v2"
)

const NotAuthErr = "not authorized"

func (app *application) requireAuth(c *fiber.Ctx) error {
	if !app.session.IsAuthenticated(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": NotAuthErr,
		})
	}

	return c.Next()
}
