package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type musicGenreHandlers struct {
	db *gorm.DB
}

func NewMusicGenreHandlers(db *gorm.DB) *musicGenreHandlers {
	return &musicGenreHandlers{db: db}
}

func (h *musicGenreHandlers) GetMusicGenreByTag(c *fiber.Ctx) error {
	var genre models.MusicGenre
	tag := c.Params("tag")

	err := h.db.Where("tag = ?", tag).First(&genre).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  genre,
	})
}

func (h *musicGenreHandlers) GetMusicGenres(c *fiber.Ctx) error {
	var genres []models.MusicGenre

	err := h.db.Find(&genres).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"items": genres,
	})
}

func (h *musicGenreHandlers) FindOrCreateMusicGenre(c *fiber.Ctx) error {
	var genre models.MusicGenre

	err := c.BodyParser(&genre)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.FirstOrCreate(&genre).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":    true,
			"messsage": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error":   false,
		"message": err,
	})
}
