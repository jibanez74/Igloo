package main

import (
	"igloo/cmd/database/models"

	"github.com/gofiber/fiber/v2"
)

func (app *config) CreateUser(c *fiber.Ctx) error {
	var user models.User

	if err := c.BodyParser(&user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	status, err := app.repo.CreateUser(&user)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(status).JSON(fiber.Map{
		"message": "User created successfully",
	})
}
