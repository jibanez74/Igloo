package main

import (
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"igloo/cmd/repository"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *config) GetMoviesWithPagination(c *fiber.Ctx) error {
	limit := c.QueryInt("id", 24)
	page := c.QueryInt("page", 1)

	count, err := app.repo.GetMovieCount()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	offset := (page - 1) * limit

	var movies []repository.SimpleMovie

	status, err := app.repo.GetMoviesWithPagination(&movies, limit, offset)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	pages := count / int64(limit)

	return c.Status(status).JSON(fiber.Map{
		"movies": movies,
		"pages":  pages,
		"page":   page,
		"count":  count,
	})
}

func (app *config) GetMovieByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var movie models.Movie
	movie.ID = uint(id)

	status, err := app.repo.GetMovieByID(&movie)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(status).JSON(fiber.Map{
		"movie": movie,
	})
}

func (app *config) CreateMovie(c *fiber.Ctx) error {
	var movie models.Movie

	err := c.BodyParser(&movie)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if movie.FilePath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "filepath is required",
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

	err = helpers.GetMovieMetadata(&movie)

	status, err := app.repo.CreateMovie(&movie)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"movie": movie,
	})
}
