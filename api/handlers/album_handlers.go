package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const defaultPageSize = 20

func (h *appHandlers) GetAlbums(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page number"})
	}

	pageSize, err := strconv.Atoi(c.Query("pageSize", strconv.Itoa(defaultPageSize)))
	if err != nil || pageSize < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page size"})
	}

	var totalAlbumsCount int64
	if err := h.db.Model(&models.Musician{}).Count(&totalAlbumsCount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	offset := (page - 1) * pageSize

	var m []models.Musician

	err = h.db.Offset(offset).Limit(pageSize).Find(&m).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err})
	}

	totalPages := int(totalAlbumsCount) / pageSize
	if int(totalAlbumsCount)%pageSize != 0 {
		totalPages++
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"items":      m,
		"totalPages": totalPages,
	})
}

func (h *appHandlers) GetAlbumByTitle(c *fiber.Ctx) error {
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

func (h *appHandlers) GetAlbumByID(c *fiber.Ctx) error {
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

func (h *appHandlers) CreateAlbum(c *fiber.Ctx) error {
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
