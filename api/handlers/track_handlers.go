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

func (h *trackHandlers) GetTracks(c *fiber.Ctx) error {
	var tracks []models.Track

	err := h.db.Find(&tracks).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"items": tracks})
}

func (h *trackHandlers) GetTrackByID(c *fiber.Ctx) error {
	var t models.Track
	id := c.Params("id")

	trackId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.db.First(&t, uint(trackId)).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": t})
}

func (h *trackHandlers) CreateTrack(c *fiber.Ctx) error {
	var t models.Track

	err := c.BodyParser(&t)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.db.Create(&t).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": t})
}
