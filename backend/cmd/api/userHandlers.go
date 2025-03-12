package main

import (
	"igloo/cmd/internal/database"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *application) getUsersPaginated(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "24"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 24
	}

	offset := (page - 1) * limit

	count, err := app.queries.GetTotalUsersCount(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	users, err := app.queries.GetUsersPaginated(c.Context(), database.GetUsersPaginatedParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	totalPages := (count + int64(limit) - 1) / int64(limit)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"items":        users,
		"current_page": page,
		"total_pages":  totalPages,
		"count":        count,
	})
}

func (app *application) createUser(c *fiber.Ctx) error {
	var req database.CreateUserParams

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	exist, err := app.queries.CheckUserExists(c.Context(), database.CheckUserExistsParams{
		Email:    req.Email,
		Username: req.Username,
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to check if user exists",
		})
	}

	if exist {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "user already exists",
		})
	}

	user, err := app.queries.CreateUser(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to create user",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user": user,
	})
}
