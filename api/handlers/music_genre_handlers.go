package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (h *appHandlers) GetMusicGenreByTag(c *fiber.Ctx) error {
	var g models.MusicGenre
	t := c.Params("tag")

	err := h.db.First(&g).Where("Tag = ?", t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": g})
}

func (h *appHandlers) GetMusicGenres(c *fiber.Ctx) error {
	var g []models.MusicGenre

	err := h.db.Find(&g).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"items": g})
}

func (h *appHandlers) FindOrCreateMusicGenre(c *fiber.Ctx) error {
	var g models.MusicGenre

	err := c.BodyParser(&g)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.db.FirstOrCreate(&g).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": g})
}
