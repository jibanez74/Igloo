package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/session"

	"github.com/gofiber/fiber/v2"
)

type loginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

const (
	authErr   = "invalid user credentials"
	serverErr = "internal server error"
)

func (app *application) login(c *fiber.Ctx) error {
	var request loginRequest

	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(request.Username) > 20 || len(request.Username) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username must be between 2 and 20 characters",
		})
	}

	if len(request.Email) < 5 || len(request.Email) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid email address",
		})
	}

	if len(request.Password) < 9 || len(request.Password) > 128 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password must be between 9 and 128 characters",
		})
	}

	user, err := app.queries.GetUserForLogin(c.Context(), database.GetUserForLoginParams{
		Email:    request.Email,
		Username: request.Username,
	})

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authErr,
		})
	}

	isMatch, err := helpers.PasswordMatches(request.Password, user.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	if !isMatch {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authErr,
		})
	}

	err = app.session.CreateAuthSession(c, &session.AuthData{
		ID:      user.ID,
		IsAdmin: user.IsAdmin,
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": fiber.Map{
			"id":       user.ID,
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"is_admin": user.IsAdmin,
			"avatar":   user.Avatar,
		},
	})
}

func (app *application) logout(c *fiber.Ctx) error {
	err := app.session.DestroyAuthSession(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "logged out successfully",
	})
}
