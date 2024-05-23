package handlers

import (
	"fmt"
	"igloo/helpers"
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *AppHandlers) GetMovieCount(c *fiber.Ctx) error {
	var count int64
	err := h.db.Model(&models.Movie{}).Count(&count).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Count": count,
	})
}

func (h *AppHandlers) GetMoviesWithPagination(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	limit, err := strconv.Atoi(c.Query("limit", "24"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	offset := (page - 1) * limit

	var count int64

	err = h.db.Model(&models.Movie{}).Count(&count).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var movies []struct {
		ID    uint
		Title string
		Thumb string
		Year  uint
	}

	err = h.db.Model(&models.Movie{}).Limit(limit).Offset(offset).Order("title asc").Select("id, title, thumb, year").Find(&movies).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	pages := int(count) / limit

	if int(count)%limit != 0 {
		pages++
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Items": fiber.Map{
			"Movies": movies,
			"Count":  count,
			"Page":   page,
			"Pages":  pages,
		},
	})
}

func (h *AppHandlers) GetMovieByID(c *fiber.Ctx) error {
	movieId, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Unable to parse params for movie id",
		})
	}

	var movie models.Movie

	err = h.db.First(&movie, uint(movieId)).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Item": movie,
	})
}

func (h *AppHandlers) CreateMovie(c *fiber.Ctx) error {
	var movie models.Movie

	err := c.BodyParser(&movie)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = h.db.Create(&movie).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	go helpers.DownloadImage(movie.Thumb, fmt.Sprintf("./public/images/movies/thumb/%s.jpg", movie.Title))

	go helpers.DownloadImage(movie.Art, fmt.Sprintf("./public/images/movies/art/%s.jpg", movie.Title))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"Item": movie,
	})
}
