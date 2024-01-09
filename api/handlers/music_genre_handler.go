package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (h *appHandler) GetMusicGenres(c *fiber.Ctx) error {
	var genres []models.MusicGenre

	result := h.db.Find(&genres).Select("ID", "Tag")
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not get music genres"})
	}

	return c.JSON(genres)
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
