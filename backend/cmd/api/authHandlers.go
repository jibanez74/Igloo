package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/gofiber/fiber/v2"
)

type loginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	if !match {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": authErr,
		})
	}

	token, err := helpers.GenerateAccessToken(&user, app.settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	ses, err := app.session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	ses.Set("user_id", user.ID)
	ses.Set("email", user.Email)
	ses.Set("username", user.Username)
	ses.Set("is_admin", user.IsAdmin)

	err = ses.Save()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"access_token": token,
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

func (app *application) logout(c *fiber.Ctx) error {
	ses, err := app.session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	err = ses.Destroy()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
	})
}

func (app *application) getAuthUser(c *fiber.Ctx) error {
	ses, err := app.session.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	id := ses.Get("user_id").(int32)

	user, err := app.queries.GetUserByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "unable to find user",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
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
