package main

import (
	"fmt"
	"igloo/cmd/internal/database/models"
	"igloo/cmd/internal/helpers"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var allowedPhotoTypes = []string{".jpg", ".jpeg", ".png", ".gif"}

type simpleUser struct {
	ID       uint
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
	IsActive bool   `json:"is_active"`
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

func (app *config) UploadUserPhoto(c *fiber.Ctx) error {
	file, err := c.FormFile("photo")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}

	if file.Size > 10*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File size too large. Maximum size is 10MB",
		})
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	err = helpers.ValidateInArray(ext, allowedPhotoTypes, "Invalid file type. Allowed types: JPG, PNG, GIF")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	filename := uuid.New().String() + ext
	uploadPath := filepath.Join(app.settings.StaticDir, "images", "avatars", filename)

	if err := c.SaveFile(file, uploadPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}

	// Get the host from the request
	host := c.Protocol() + "://" + c.Hostname()

	// Return the file path with full URL
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"filename": filename,
		"path":     fmt.Sprintf("%s/api/v1/static/images/avatars/%s", host, filename),
	})
}
