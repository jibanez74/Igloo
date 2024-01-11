package handlers

import (
	"igloo/models"
	"igloo/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type albumHandler struct {
	repo repository.AlbumRepo
}

func NewAlbumHandler(repo repository.AlbumRepo) *albumHandler {
	return &albumHandler{repo}
}

func (h *albumHandler) GetAlbums(c *fiber.Ctx) error {
	var a []models.Album

	err := h.repo.GetAlbums(&a)
	if err != nil && err == gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error.  Unable to get albums"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"items": a})
}
