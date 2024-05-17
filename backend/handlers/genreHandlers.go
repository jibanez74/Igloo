package handlers

import (
	"igloo/helpers"
	"igloo/models"

	"github.com/gofiber/fiber/v2"
)

func (h *AppHandlers) FindOrCreateGenre(c *fiber.Ctx) error {
	var genre models.Genre

	err := c.BodyParser(&genre)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Unable to parse genre from json body",
		})
	}

	err = h.db.Where("tag = ?", genre.Tag).FirstOrCreate(&genre).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"item": genre,
	})
}
