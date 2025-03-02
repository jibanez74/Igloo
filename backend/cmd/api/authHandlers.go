package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tokens"

	"github.com/gofiber/fiber/v2"
)

type loginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

const (
	authErr   = "invalid credentials"
	serverErr = "server error"
)

func (app *application) login(c *fiber.Ctx) error {
	var req loginRequest

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing required fields",
		})
	}

	user, err := app.queries.GetUserForLogin(c.Context(), database.GetUserForLoginParams{
		Email:    req.Email,
		Username: req.Username,
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": authErr,
		})
	}

	match, err := helpers.PasswordMatches(req.Password, user.Password)
	if err != nil {
		app.logger.Error(fmt.Errorf("error checking password: %w", err))

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	if !match {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authErr,
		})
	}

	tokenPair, err := app.tokens.GenerateTokenPair(tokens.Claims{
		UserID:   int(user.ID),
		Username: user.Username,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
	})

	if err != nil {
		app.logger.Error(fmt.Errorf("error generating tokens: %w", err))

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"tokens": tokenPair,
		"user": fiber.Map{
			"id":       user.ID,
			"name":     user.Name,
			"email":    user.Email,
			"username": user.Username,
			"avatar":   user.Avatar,
			"isAdmin":  user.IsAdmin,
		},
	})
}
