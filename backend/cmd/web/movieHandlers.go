package main

import (
	"igloo/cmd/internal/database/models"
	"igloo/cmd/internal/helpers"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type simpleMovie struct {
	ID    uint
	Title string `json:"title"`
	Year  uint   `json:"year"`
	Thumb string `json:"thumb"`
}

func (app *config) getLatestMovies(c *fiber.Ctx) error {
	var movies []simpleMovie

	err := app.db.Model(&models.Movie{}).Select("id, title, thumb, year").Order("created_at DESC").Limit(12).Find(&movies).Error
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movies": movies,
	})
}

func (app *config) getMovieByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var movie models.Movie

	err = app.db.First(&movie, uint(id)).Error
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movie": movie,
	})
}

func (app *config) createMovie(c *fiber.Ctx) error {
	var movie models.Movie

	err := c.BodyParser(&movie)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if movie.FilePath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "movie file path is required",
		})
	}

	if movie.TmdbID != "" {
		err = app.tmdb.GetTmdbMovieByID(&movie)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	err = helpers.GetMovieMetadata(&movie, app.settings.Ffprobe)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = app.db.Create(&movie).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"movie": movie,
	})
}

func (app *config) directStreamMovie(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var movie models.Movie

	err = app.db.First(&movie, uint(id)).Error
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	file, err := os.Open(movie.FilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	defer file.Close()

	c.Set("Content-Type", movie.ContentType)
	c.Set("Content-Length", strconv.FormatInt(movie.Size, 10))
	c.Set("Content-Disposition", "inline; filename="+movie.Title)
	c.Set("Accept-Ranges", "bytes")

	return c.SendFile(movie.FilePath)
}
