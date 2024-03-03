package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type trackHandlers struct {
	db *gorm.DB
}

func NewTrackHandlers(db *gorm.DB) *trackHandlers {
	return &trackHandlers{db: db}
}

func (h *trackHandlers) GetTrackByID(c *fiber.Ctx) error {
	var track models.Track
	id := c.Params("id")

	trackId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.First(&track, uint(trackId)).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  track,
	})
}

func (h *trackHandlers) CreateTrack(c *fiber.Ctx) error {
	var track models.Track

	err := c.BodyParser(&track)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.Create(&track).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"error": false,
		"item":  track,
	})
}
