package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *AppHandlers) GetArtistByID(c *fiber.Ctx) error {
	artistId, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var artist models.Artist

	err = h.db.First(&artist, uint(artistId)).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"item": artist,
	})
}

func (h *AppHandlers) FindOrCreateArtist(c *fiber.Ctx) error {
	var artist models.Artist

	err := c.BodyParser(&artist)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = h.db.Where("name = ?", artist.Name).FirstOrCreate(&artist).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"item": artist,
	})
}
