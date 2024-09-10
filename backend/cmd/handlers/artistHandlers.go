package handlers

import (
	"igloo/cmd/models"

	"github.com/gofiber/fiber/v2"
)

func (h *Handlers) FindOrCreateArtist(c *fiber.Ctx) error {
	var artist models.Artist

	err := c.BodyParser(&artist)
	if err != nil {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	err = h.repo.FindOrCreateArtist(&artist)
	if err != nil {
		c.Status(getStatusCode(err)).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Artist": artist,
	})
}
