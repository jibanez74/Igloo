package main

import (
	"igloo/cmd/database/models"
	"log"

	"github.com/gofiber/fiber/v2"
)

const authError = "invalid credentials"
const serverError = "server error"

func (app *config) Login(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		log.Printf("Login error: Failed to parse request body: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// Validate required fields
	if req.Username == "" && req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username or email is required",
		})
	}

	if req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password is required",
		})
	}

	var user models.User
	user.Username = req.Username
	user.Email = req.Email

	if err := app.Repo.GetAuthUser(&user); err != nil {
		log.Printf("Login error: Failed to get user: %v\n", err)
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

	// Check if user is active
	if !user.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Account is inactive",
		})
	}

	// Get existing session if any and destroy it
	if sess, err := app.Session.Get(c); err == nil {
		sess.Destroy()
	}

	// Create new session
	session, err := app.Session.Get(c)
	if err != nil {
		log.Printf("Login error: Failed to create session: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverError,
		})
	}

	session.Set("id", user.ID)
	session.Set("is_admin", user.IsAdmin)

	if err := session.Save(); err != nil {
		log.Printf("Login error: Failed to save session: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverError,
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

func (app *config) Logout(c *fiber.Ctx) error {
	session, err := app.Session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "server error",
		})
	}

	session.Destroy()

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "logged out",
	})
}
