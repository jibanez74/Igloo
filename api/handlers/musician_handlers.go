package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type musicianHandlers struct {
	db *gorm.DB
}

func NewMusicianHandlers(db *gorm.DB) *musicianHandlers {
	return &musicianHandlers{db: db}
}

func (h *musicianHandlers) GetMusicianByID(c *fiber.Ctx) error {
	var m models.Musician
	id := c.Params("id")

	musicianId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.db.First(&m, uint(musicianId)).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": m})
}

func (h *musicianHandlers) GetMusicianByName(c *fiber.Ctx) error {
	var m models.Musician
	n := c.Params("name")

	err := h.db.First(&m).Where("Name = ?", n).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": m})
}

func (h *musicianHandlers) CreateMusician(c *fiber.Ctx) error {
	var m models.Musician

	err := c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "An error occurred parsing post data"})
	}

	err = h.db.Create(&m).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Unable to create musician in data base"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": m})
}
