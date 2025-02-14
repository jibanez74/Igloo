package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *application) getTotalMovieCount(c *fiber.Ctx) error {
	count, err := app.queries.GetMovieCount(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"count": count,
	})
}

func (app *application) getLatestMovies(c *fiber.Ctx) error {
	movies, err := app.queries.GetLatestMovies(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movies": movies,
	})
}

func (app *application) getMovieDetails(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to parse id %s", c.Params("id")),
		})
	}

	movie, err := app.queries.GetMovieDetails(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movie": movie,
	})
}

func (app *application) getMoviesPaginated(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "24"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 24
	}

	offset := (page - 1) * limit

	movies, err := app.queries.GetMoviesPaginated(c.Context(), database.GetMoviesPaginatedParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	totalMovies, err := app.queries.GetMovieCount(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	totalPages := (totalMovies + int64(limit) - 1) / int64(limit)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movies":       movies,
		"current_page": page,
		"total_pages":  totalPages,
		"total_movies": totalMovies,
	})
}

func (app *application) streamMovie(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to parse id %s", c.Params("id")),
		})
	}

	movie, err := app.queries.GetMovieForStreaming(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("movie with id %d not found", id),
		})
	}

	_, err = os.Stat(movie.FilePath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "movie file not found",
		})
	}

	c.Set("Content-Type", movie.ContentType)
	c.Set("Content-Length", fmt.Sprintf("%d", movie.Size))
	c.Set("Accept-Ranges", "bytes")
	c.Set("Cache-Control", "public, max-age=31536000")

	rangeHeader := c.Get("Range")

	if rangeHeader != "" {
		ranges, err := helpers.ParseRange(rangeHeader, movie.Size)
		if err != nil {
			return c.Status(fiber.StatusRequestedRangeNotSatisfiable).JSON(fiber.Map{
				"error": "invalid range request",
			})
		}

		if len(ranges) > 0 {
			r := ranges[0]

			c.Status(fiber.StatusPartialContent)
			c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, movie.Size))
			c.Set("Content-Length", fmt.Sprintf("%d", r.End-r.Start+1))

			return c.SendFile(movie.FilePath, true)
		}
	}

	return c.SendFile(movie.FilePath, true)
}
