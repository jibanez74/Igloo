package main

import (
	"igloo/cmd/internal/database/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type simpleUser struct {
	ID       uint
	Name     string
	Username string
	Email    string
	IsAdmin  bool
	IsActive bool
}

func (app *config) CreateUser(c *fiber.Ctx) error {
	var user models.User

	err := c.BodyParser(&user)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = app.db.Create(&user).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user": user,
	})
}

func (app *config) GetUsers(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "10"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	offset := (page - 1) * limit

	var count int64

	err = app.db.Model(&models.User{}).Count(&count).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var users []simpleUser

	err = app.db.Model(&models.User{}).
		Select("id, name, username, email, is_admin, is_active, thumb, created_at").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&users).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	totalPages := (count + int64(limit) - 1) / int64(limit)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"users": users,
		"count": count,
		"page":  page,
		"pages": totalPages,
		"limit": limit,
	})
}
