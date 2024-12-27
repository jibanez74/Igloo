package main

import (
	"errors"
	"igloo/cmd/database/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const errMsg = "invalid credentials"
const internalErr = "unable to process your request"

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
	user.Email = req.Email
	user.Username = req.Username

	err = app.repo.GetAuthUser(&user)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": errMsg,
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": internalErr,
		})
	}

	match, err := user.PasswordMatches(req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": internalErr,
		})
	}

	if !match {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": errMsg,
		})
	}

	ses, err := app.session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": internalErr,
		})
	}

	ses.Set("is_admin", user.IsAdmin)
	ses.Set("id", user.ID)

	err = ses.Save()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": fiber.Map{
			"name":     user.Name,
			"username": user.Username,
			"email":    user.Email,
			"isAdmin":  user.IsAdmin,
		},
	})
}

func (app *config) Logout(c *fiber.Ctx) error {
	ses, err := app.session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = ses.Destroy()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}
