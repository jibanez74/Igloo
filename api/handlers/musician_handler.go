package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (h *appHandler) GetMusicianWithPagination(c *fiber.Ctx) error {
	var musicians []models.Musician
	pageStr := c.Query("page", "1")
	limitStr := c.Query("limit", "30")

	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid page parameter"})
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid limit parameter"})
	}

	offset := (page - 1) * limit

	err = h.db.Offset(offset).Limit(limit).Find(&musicians).Error
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Musician not found"})
	}

	var totalMusicians int64
	h.db.Model(&models.Musician{}).Count(&totalMusicians)

	totalPages := int(totalMusicians) / limit
	if totalMusicians%int64(limit) != 0 {
		totalPages++
	}

	response := fiber.Map{
		"musicians":  musicians,
		"pagination": fiber.Map{"page": page, "totalPages": totalPages, "totalItems": totalMusicians},
	}

	return c.JSON(response)
}

func (h *appHandler) GetMusicianByName(c *fiber.Ctx) error {
	var musician models.Musician
	name := c.Params("name")

	err := h.db.Where("Name = ?", name).First(&musician).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Musician not found"})
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Musician not found"})
	}

	return c.JSON(musician)
}

func (h *appHandler) GetMusicianById(c *fiber.Ctx) error {
	var musician models.Musician
	id := c.Params("id")

	err := h.db.First(&musician, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Musician not found"})
		}

		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Musician not found"})
	}

	return c.JSON(musician)
}

func (h *appHandler) CreateMusician(c *fiber.Ctx) error {
	var musician models.Musician

	if err := c.BodyParser(&musician); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := h.db.Create(&musician).Error
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(musician)
}

func (h *appHandler) DeleteMusician(c *fiber.Ctx) error {
	id := c.Params("id")

	err := h.db.Delete(&models.Musician{}, id).Error
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
