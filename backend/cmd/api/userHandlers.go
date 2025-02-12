package main

import (
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *application) getUserByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	user, err := app.queries.GetUserByID(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"user": user,
	})
}

func (app *application) createUser(c *fiber.Ctx) error {
	var request database.CreateUserParams

	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse request body",
		})
	}

	if len(request.Name) > 60 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid name length",
		})
	}

	if len(request.Email) > 100 || len(request.Email) < 5 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid email address",
		})
	}

	if len(request.Username) > 20 || len(request.Username) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid username length",
		})
	}

	if len(request.Password) > 128 || len(request.Password) < 9 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid password length",
		})
	}

	hashPassword, err := helpers.HashPassword(request.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	request.Password = hashPassword

	_, err = app.queries.CreateUser(c.Context(), request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "user created successfully",
	})
}
