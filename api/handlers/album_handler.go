package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *appHandler) GetAlbumsWithPagination(c *fiber.Ctx) error {
	var albums []models.Album
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

	err = h.db.Offset(offset).Limit(limit).Find(&albums).Error
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Musician not found"})
	}

	var totalAlbums int64
	h.db.Model(&models.Album{}).Count(&totalAlbums)

	totalPages := int(totalAlbums) / limit
	if totalAlbums%int64(limit) != 0 {
		totalPages++
	}

	response := fiber.Map{
		"albums":     albums,
		"pagination": fiber.Map{"page": page, "totalPages": totalPages, "totalItems": totalAlbums},
	}

	return c.JSON(response)
}

func (h *appHandler) GetAlbumByTitle(c *fiber.Ctx) error {
	var album models.Album

	if err := h.db.Where("title = ?", c.Params("title")).First(&album).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Album not found"})
	}

	return c.JSON(album)
}

func (h *appHandler) GetAlbumById(c *fiber.Ctx) error {
	var album models.Album

	if err := h.db.First(&album, c.Params("id")).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Album not found"})
	}

	return c.JSON(album)
}

func (h *appHandler) CreateAlbum(c *fiber.Ctx) error {
	var album models.Album

	if err := c.BodyParser(&album); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if err := h.db.Create(&album).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not create album"})
	}

	return c.JSON(album)
}
