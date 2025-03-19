package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (app *application) login(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

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

	cookie := new(fiber.Cookie)
	cookie.Name = "access_token"
	cookie.Value = token
	cookie.Expires = time.Now().Add(helpers.TokenExpiry)
	cookie.HTTPOnly = true
	cookie.Secure = true
	cookie.SameSite = "Strict"
	cookie.Domain = app.settings.CookieDomain
	cookie.Path = app.settings.CookiePath

	c.Cookie(cookie)

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

func (app *application) getCurrentUser(c *fiber.Ctx) error {
	userIDStr := c.Locals("userID").(string)
	if userIDStr == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": notAuthMsg})
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	user, err := app.queries.GetUserByID(c.Context(), int32(userID))
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
			"avatar":   user.Avatar,
			"isAdmin":  user.IsAdmin,
		},
	})
}

func (app *application) logout(c *fiber.Ctx) error {
	cookie := new(fiber.Cookie)
	cookie.Name = "access_token"
	cookie.Value = ""
	cookie.Expires = time.Now().Add(-1 * time.Hour) // Set to expire in the past
	cookie.HTTPOnly = true
	cookie.Secure = true
	cookie.SameSite = "Strict"
	cookie.Domain = app.settings.CookieDomain
	cookie.Path = app.settings.CookiePath

	c.Cookie(cookie)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Successfully logged out",
	})
}
