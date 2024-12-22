package main

import "github.com/gofiber/fiber/v2"

func (app *config) isAuth(c *fiber.Ctx) error {
	ses, err := app.session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	id := ses.Get("id")
	if id == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "not authorized",
		})
	}

	return c.Next()
}
