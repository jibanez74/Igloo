package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *AppHandlers) GetStudioByID(c *fiber.Ctx) error {
	studioId, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Unable to parse params for studio id",
		})
	}

	var studio struct {
		Name    string `json:"name"`
		Country string `json:"country"`
		Thumb   string `json:"thumb"`
	}

	err = h.db.Model(&models.Studio{}).Select("name, country, thumb").First(&studio, uint(studioId)).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"item": studio,
	})
}

func (h *AppHandlers) CreateStudio(c *fiber.Ctx) error {
	var studio models.Studio

	err := c.BodyParser(&studio)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = h.db.Create(&studio).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"item": studio,
	})
}
