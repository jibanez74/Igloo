package main

import (
	"igloo/cmd/internal/database/models"
	"log"

	"github.com/gofiber/fiber/v2"
)

const (
	authError   = "invalid credentials"
	serverError = "server error"
)

func (app *config) login(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	var user models.User
	user.Username = req.Username
	user.Email = req.Email

	err = app.db.Where("email = ? AND username = ?", req.Email, req.Username).First(&user).Error
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authError,
		})
	}

	isMatch, err := user.PasswordMatches(req.Password)
	if err != nil {
		log.Printf("Login error: Failed to verify password: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverError,
		})
	}

	if !isMatch {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authError,
		})
	}

	if !user.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Account is inactive",
		})
	}

	session, err := app.store.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverError,
		})
	}

	session.Set("user_id", user.ID)
	session.Set("is_admin", user.IsAdmin)

	err = session.Save()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverError,
		})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "is_authenticated",
		Value:    "true",
		Path:     "/",
		MaxAge:   24 * 60 * 60, // 24 hours
		Secure:   true,
		HTTPOnly: false, // Allows JavaScript to detect login state
		SameSite: "Strict",
	})

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

func (app *config) getAuthUser(c *fiber.Ctx) error {
	user := GetUserFromContext(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": errNotAuth,
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
