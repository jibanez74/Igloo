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

	count, err := app.queries.GetMovieCount(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	totalPages := (count + int64(limit) - 1) / int64(limit)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"items":        movies,
		"current_page": page,
		"total_pages":  totalPages,
		"count":        count,
	})
}

func (app *application) streamMovie(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse movie id from url params",
		})
	}

	movie, err := app.queries.GetMovieForDirectPlayback(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to find movie with id %d", id),
		})
	}

	c.Set("Accept-Ranges", "bytes")
	c.Set("Content-Type", movie.ContentType)

	rangeHdr := c.Get("Range")

	if rangeHdr != "" {
		start, end, err := helpers.ParseRange(rangeHdr, movie.Size)
		if err != nil {
			return c.Status(fiber.StatusRequestedRangeNotSatisfiable).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if end >= movie.Size {
			end = movie.Size - 1
		}

		length := end - start + 1

		file, err := os.Open(movie.FilePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "unable to open movie file",
			})
		}
		defer file.Close()

		c.Status(fiber.StatusPartialContent)
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, movie.Size))
		c.Set("Content-Length", strconv.FormatInt(length, 10))

		return c.SendStream(file, int(length))
	}

	c.Set("Content-Length", strconv.FormatInt(movie.Size, 10))

	return c.SendFile(movie.FilePath)
}
