package main

import (
	"igloo/cmd/database/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *config) isAuth(c *fiber.Ctx) error {
	ses, err := app.session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Session error",
		})
	}

	id := ses.Get("id")
	if id == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Please log in to continue",
		})
	}

	idStr, ok := id.(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid session format",
		})
	}

	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid session data",
		})
	}

	var user models.User
	user.ID = uint(userID)

	status, err := app.repo.GetUserByID(&user)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": "Authentication failed",
		})
	}

	if !user.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Account is inactive",
		})
	}

	c.Locals("user", user)

	if err := ses.Save(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Session error",
		})
	}

	return c.Next()
}
