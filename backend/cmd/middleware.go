package main

import (
	"igloo/cmd/database/models"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *config) isAuth(c *fiber.Ctx) error {
	ses, err := app.session.Get(c)
	if err != nil {
		log.Println(err.Error())
		log.Println("unable to get session")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to get session",
		})
	}

	id := ses.Get("id")
	if id == nil {
		log.Println("no id was found in session")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Please log in to continue",
		})
	}

	idStr, ok := id.(string)
	if !ok {
		log.Println("unable to convert id to string")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid session format",
		})
	}

	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		log.Println(err.Error())
		log.Println("unable to parse id with strconv")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid session data",
		})
	}

	var user models.User
	user.ID = uint(userID)

	status, err := app.repo.GetUserByID(&user)
	if err != nil {
		log.Println(err.Error())
		log.Println("unable to get user from db")
		return c.Status(status).JSON(fiber.Map{
			"error": "Authentication failed",
		})
	}

	if !user.IsActive {
		log.Println("the user is not active")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Account is inactive",
		})
	}

	c.Locals("user", user)

	return c.Next()
}
