package main

import (
	"igloo/cmd/database/models"

	"github.com/gofiber/fiber/v2"
)

func (app *config) GetAuthUser(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": fiber.Map{
			"ID":       user.ID,
			"Name":     user.Name,
			"Username": user.Username,
			"Email":    user.Email,
			"IsAdmin":  user.IsAdmin,
			"thumb":    user.Thumb,
		},
	})
}
