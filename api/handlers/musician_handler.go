package handlers

import (
	"igloo/models"
	"igloo/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type musicianHandler struct {
	repo repository.MusicianRepo
}

func NewMusicianHandler(repo repository.MusicianRepo) *musicianHandler {
	return &musicianHandler{repo}
}

func (h *musicianHandler) GetMusicians(c *fiber.Ctx) error {
	var m []models.Musician

	err := h.repo.GetMusicians(&m)
	if err != nil && err == gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"items": m})
}

func (h *musicianHandler) GetUserByName(c *fiber.Ctx) error {
	var m models.Musician
	name := c.Params("name")

	err := h.repo.GetMusicianByName(&m, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Unable to find musician in data base"})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": m})
}

func (h *musicianHandler) CreateMusician(c *fiber.Ctx) error {
	var m models.Musician

	err := c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.repo.CreateMusician(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": m})
}
