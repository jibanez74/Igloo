package main

import (
	"igloo/cmd/internal/database/models"

	"github.com/gofiber/fiber/v2"
)

const (
	userContextKey = "user"
	errNotAuth     = "Not Authorized"
)

func (app *config) requireAuth(c *fiber.Ctx) error {
	sess, err := app.store.Get(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": errNotAuth,
		})
	}

	userID := sess.Get("user_id")
	if userID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": errNotAuth,
		})
	}

	var user models.User

	err = app.db.First(&user, userID).Error
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": errNotAuth,
		})
	}

	c.Locals(userContextKey, user)

	return c.Next()
}

// GetUserFromContext retrieves the user from the fiber context
func GetUserFromContext(c *fiber.Ctx) *models.User {
	user, ok := c.Locals(userContextKey).(models.User)
	if !ok {
		return nil
	}
	return &user
}
