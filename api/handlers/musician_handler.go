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
