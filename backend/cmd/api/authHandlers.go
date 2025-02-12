package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/session"
	"log"

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
		log.Println("unable to parse body")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if len(request.Username) > 20 || len(request.Username) < 2 {
		log.Println("invalid username length")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username must be between 2 and 20 characters",
		})
	}

	if len(request.Email) < 5 || len(request.Email) > 100 {
		log.Println("invalid email length")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid email address",
		})
	}

	if len(request.Password) < 9 || len(request.Password) > 128 {
		log.Println("invalid password length")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password must be between 9 and 128 characters",
		})
	}

	user, err := app.queries.GetUserForLogin(c.Context(), database.GetUserForLoginParams{
		Email:    request.Email,
		Username: request.Username,
	})

	if err != nil {
		log.Println("unable to get user")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authErr,
		})
	}

	isMatch, err := helpers.PasswordMatches(request.Password, user.Password)
	if err != nil {
		log.Println("unable to run match func")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	if !isMatch {
		log.Println("passwords do not match")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authErr,
		})
	}

	err = app.session.CreateAuthSession(c, &session.AuthData{
		ID:      user.ID,
		IsAdmin: user.IsAdmin,
	})

	if err != nil {
		log.Println("unable to create auth session")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": fiber.Map{
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
