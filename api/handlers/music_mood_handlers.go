package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type musicMoodHandlers struct {
	db *gorm.DB
}

func NewMusicMoodHandlers(db *gorm.DB) *musicMoodHandlers {
	return &musicMoodHandlers{db: db}
}

func (h *musicMoodHandlers) GetMusicMoodByTag(c *fiber.Ctx) error {
	var mood models.MusicMood
	tag := c.Params("tag")

	err := h.db.Where("tag = ?", tag).First(&mood).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  mood,
	})
}

func (h *musicMoodHandlers) GetMusicMoods(c *fiber.Ctx) error {
	var moods []models.MusicMood

	err := h.db.Find(&moods).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"items": moods,
	})
}

func (h *musicMoodHandlers) FindOrCreateMusicMood(c *fiber.Ctx) error {
	var mood models.MusicMood

	err := c.BodyParser(&mood)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.FirstOrCreate(&mood).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error":   false,
		"message": err,
	})
}
