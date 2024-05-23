package handlers

import (
	"igloo/helpers"
	"igloo/models"

	"github.com/gofiber/fiber/v2"
)

func (h *AppHandlers) GetRecent(c *fiber.Ctx) error {
	const limit = 12

	var movies []struct {
		ID    uint
		Title string
		Thumb string
		Year  uint
	}

	err := h.db.Model(&models.Movie{}).Select("id, title, thumb, year").Order("created_at desc").Limit(limit).Find(&movies).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Items": fiber.Map{
			"Movies": movies,
		},
	})
}
