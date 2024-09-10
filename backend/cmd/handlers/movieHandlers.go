package handlers

import (
	"igloo/cmd/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *Handlers) GetMovieCount(c *fiber.Ctx) error {
	var count int64

	err := h.repo.GetMovieCount(&count)
	if err != nil {
		c.Status(getStatusCode(err)).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Count": count,
	})
}

func (h *Handlers) GetLatestMovies(c *fiber.Ctx) error {
	var movies [12]models.RecentMovie

	err := h.repo.GetLatestMovies(&movies)
	if err != nil {
		c.Status(getStatusCode(err)).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Movies": movies,
	})
}

func (h *Handlers) GetMovieByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	var movie models.Movie

	err = h.repo.GetMovieByID(&movie, uint(id))
	if err != nil {
		c.Status(getStatusCode(err)).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Movie": movie,
	})
}

func (h *Handlers) CreateMovie(c *fiber.Ctx) error {
	var movie models.Movie

	err := c.BodyParser(&movie)
	if err != nil {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	err = h.repo.CreateMovie(&movie)
	if err != nil {
		c.Status(getStatusCode(err)).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"Movie": movie,
	})
}
