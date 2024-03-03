package handlers

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type movieHandlers struct {
	db *gorm.DB
}

func NewMovieHandlers(db *gorm.DB) *movieHandlers {
	return &movieHandlers{db: db}
}

func (h *movieHandlers) GetMoviesWithPagination(c *fiber.Ctx) error {
	var movies []models.Movie
	var count int64
	const limit = 24

	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid page number",
		})
	}

	offset := (page - 1) * limit

	err = h.db.Model(&movies).Count(&count).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.Limit(limit).Offset(offset).Order("id desc").Find(&movies).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	pages := int(count) / limit

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"items": movies,
		"count": count,
		"pages": pages,
	})
}

func (h *movieHandlers) GetMovieByID(c *fiber.Ctx) error {
	var movie models.Movie
	id := c.Params("id")

	movieId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.Find(&movie, uint(movieId)).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  movie,
	})
}

func (h *movieHandlers) CreateMovie(c *fiber.Ctx) error {
	var movie models.Movie

	err := c.BodyParser(&movie)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.Create(&movie).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"error": false,
		"item":  movie,
	})
}

func (h *movieHandlers) UpdateMovie(c *fiber.Ctx) error {
	var movie models.Movie

	err := c.BodyParser(&movie)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.Save(&movie).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  movie,
	})
}
