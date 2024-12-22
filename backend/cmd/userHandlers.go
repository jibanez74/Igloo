package main

import (
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *config) GetAuthenticatedUser(c *fiber.Ctx) error {
	userID, ok := helpers.UserIDFromContext(c.Context())
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var user models.User
	user.ID = uint(id)

	status, err := app.repo.GetUserByID(&user)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": fiber.Map{
			"id":       user.ID,
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"isAdmin":  user.IsAdmin,
			"thumb":    user.Thumb,
		},
	})
}

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
