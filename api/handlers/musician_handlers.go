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
	var musician models.Musician
	id := c.Params("id")

	musicianId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.First(&musician, uint(musicianId)).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  musician,
	})
}

func (h *musicianHandlers) GetMusicianByName(c *fiber.Ctx) error {
	var musician models.Musician
	name := c.Params("name")

	err := h.db.Where("name = ?", name).First(&musician).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  musician,
	})
}

func (h *musicianHandlers) CreateMusician(c *fiber.Ctx) error {
	var musician models.Musician

	err := c.BodyParser(&musician)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.Create(&musician).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"error": false,
		"item":  musician,
	})
}
