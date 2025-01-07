package main

import (
	"igloo/cmd/database/models"

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

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var user models.User
	user.Username = req.Username
	user.Email = req.Email

	err = app.Repo.GetAuthUser(&user)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authError,
		})
	}

	isMatch, err := user.PasswordMatches(req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "server error",
		})
	}

	if !isMatch {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authError,
		})
	}

	session, err := app.Session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "server error",
		})
	}

	session.Set("id", user.ID)
	session.Set("is_admin", user.IsAdmin)

	err = session.Save()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "server error",
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
