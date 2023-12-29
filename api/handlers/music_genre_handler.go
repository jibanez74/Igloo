package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
)

func (h *appHandler) GetMusicGenres(c *fiber.Ctx) error {
	var musicGenres []models.MusicGenre
	if err := h.db.Find(&musicGenres).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Music genres not found"})
	}

	return c.JSON(musicGenres)
}

func (h *appHandler) FindOrCreateMusicGenre(c *fiber.Ctx) error {
	var musicGenre models.MusicGenre

	if err := c.BodyParser(&musicGenre); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	err := h.db.Where(models.MusicGenre{Tag: musicGenre.Tag}).FirstOrCreate(&musicGenre).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not find or create music genre"})
	}

	return c.JSON(musicGenre)
}
