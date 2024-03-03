package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type albumHandlers struct {
	db *gorm.DB
}

func NewAlbumHandlers(db *gorm.DB) *albumHandlers {
	return &albumHandlers{db: db}
}

func (h *albumHandlers) GetAlbumByTitle(c *fiber.Ctx) error {
	var album models.Album
	title := c.Params("title")

	err := h.db.Where("title = ?", title).First(&album).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  album,
	})
}

func (h *albumHandlers) GetAlbumByID(c *fiber.Ctx) error {
	var album models.Album
	id := c.Params("id")

	albumId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.First(&album, uint(albumId)).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  album,
	})
}

func (h *albumHandlers) CreateAlbum(c *fiber.Ctx) error {
	var album models.Album

	err := c.BodyParser(&album)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.Create(&album).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"error": false,
		"item":  album,
	})
}
