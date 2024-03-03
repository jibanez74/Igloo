package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type albumHandlers struct {
	db *gorm.DB
}

func NewAlbumHandlers(db *gorm.DB) *albumHandlers {
	return &albumHandlers{db: db}
}

func (h *albumHandlers) GetAlbumByTitle(c *fiber.Ctx) error {
	var a models.Album
	t := c.Params("title")

	err := h.db.First(&a).Where("title = ?", t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": a})
}

func (h *albumHandlers) GetAlbumByID(c *fiber.Ctx) error {
	var a models.Album
	id := c.Params("id")

	albumId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.db.First(&a, uint(albumId)).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"item": a})
}

func (h *albumHandlers) CreateAlbum(c *fiber.Ctx) error {
	var a models.Album

	err := c.BodyParser(&a)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err})
	}

	err = h.db.Create(&a).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"item": a})
}
