package main

import (
	"igloo/cmd/database/models"

	"github.com/gofiber/fiber/v2"
)

func (app *config) GetAuthUser(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid user data in session",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": fiber.Map{
			"ID":       user.ID,
			"name":     user.Name,
			"username": user.Username,
			"email":    user.Email,
			"isAdmin":  user.IsAdmin,
			"thumb":    user.Thumb,
		},
	})
}
