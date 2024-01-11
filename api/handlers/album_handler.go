package handlers

import (
	"igloo/models"
	"igloo/repository"
	"strconv"

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

func (h *albumHandler) GetAlbumByID(c *fiber.Ctx) error {
	var a models.Album
	idParam := c.Params("id")

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid ID format",
		})
	}

	err = h.repo.GetAlbumByID(&a, uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": a})
}

func (h *albumHandler) GetAlbumByTitle(c *fiber.Ctx) error {
	var a models.Album
	t := c.Params("title")

	err := h.repo.GetAlbumByTitle(&a, t)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": a})
}

func (h *albumHandler) CreateAlbum(c *fiber.Ctx) error {
	var a models.Album

	err := h.repo.CreateAlbum(&a)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": a})
}
