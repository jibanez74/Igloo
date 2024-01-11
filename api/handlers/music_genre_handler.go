package handlers

import (
	"igloo/models" // Import the models package
	"igloo/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type musicGenreHandler struct {
	repo repository.MusicGenreRepo
}

func NewMusicGenreHandler(repo repository.MusicGenreRepo) *musicGenreHandler {
	return &musicGenreHandler{repo}
}

func (h *musicGenreHandler) GetMusicGenres(c *fiber.Ctx) error {
	var genres []models.MusicGenre

	err := h.repo.GetMusicGenres(&genres)
	if err != nil && err != gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"items": genres})
}

func (h *musicGenreHandler) CreateMusicGenre(c *fiber.Ctx) error {
	var genre models.MusicGenre

	err := c.BodyParser(&genre)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.repo.CreateMusicGenre(&genre)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": genre})
}
