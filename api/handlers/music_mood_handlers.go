package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (h *appHandlers) GetMusicMoodByTag(c *fiber.Ctx) error {
	var m models.MusicMood
	t := c.Params("tag")

	err := h.db.First(&m).Where("tag = ?", t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": m})
}

func (h *appHandlers) GetMusicMoods(c *fiber.Ctx) error {
	var m []models.MusicMood

	err := h.db.Find(&m).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"items": m})
}

func (h *appHandlers) FindOrCreateMusicMood(c *fiber.Ctx) error {
	var m models.MusicMood

	err := c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.db.FirstOrCreate(&m).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": m})
}
