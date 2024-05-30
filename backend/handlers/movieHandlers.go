package handlers

import (
	"fmt"
	"igloo/helpers"
	"igloo/models"
	"os"
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

	err = h.db.Preload("CrewList").Preload("CastList").Preload("Studios").Preload("Genres").Preload("ChapterList").Preload("VideoList").Preload("AudioList").Preload("SubtitleList").First(&movie, uint(movieId)).Error
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

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"Item": movie,
	})
}

func (h *AppHandlers) DeleteMovie(c *fiber.Ctx) error {
	movieId, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Unable to parse params for movie id",
		})
	}

	err = h.db.Delete(&models.Movie{}, uint(movieId)).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).JSON(fiber.Map{})
}

func (h *AppHandlers) DirectPlayMovie(c *fiber.Ctx) error {
	options := c.Queries()

	filePath := options["file_path"]
	if filePath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File path is required",
		})
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) || os.IsPermission(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "File not found or not readable",
		})
	}

	size, err := strconv.ParseInt(options["size"], 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid or missing size",
		})
	}

	videoContainer := options["container"]
	if videoContainer == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container type is required",
		})
	}

	rangeHeader := c.Get("Range")
	if rangeHeader == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Range header is required",
		})
	}

	chunkSize := int64(10 * 1024 * 1024)
	start, end := helpers.ParseRange(rangeHeader, size, chunkSize)

	contentLength := end - start + 1

	c.Set("Content-Type", "video/"+videoContainer)
	c.Set("Accept-Ranges", "bytes")
	c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))

	return c.SendFile(filePath)
}
