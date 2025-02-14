package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type httpRange struct {
	start, end int64
}

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

	c.Set("Content-Type", movie.ContentType)
	c.Set("Content-Length", fmt.Sprintf("%d", movie.Size))
	c.Set("Accept-Ranges", "bytes")

	rangeHeader := c.Get("Range")

	if rangeHeader != "" {
		ranges, err := parseRange(rangeHeader, movie.Size)
		if err != nil {
			return c.Status(fiber.StatusRequestedRangeNotSatisfiable).JSON(fiber.Map{
				"error": "invalid range request",
			})
		}

		if len(ranges) > 0 {
			r := ranges[0]

			c.Status(fiber.StatusPartialContent)
			c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", r.start, r.end, movie.Size))
			c.Set("Content-Length", fmt.Sprintf("%d", r.end-r.start+1))

			return c.SendFile(movie.FilePath, true)
		}
	}

	return c.SendFile(movie.FilePath, true)
}

func parseRange(rangeHeader string, size int64) ([]httpRange, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("invalid range format")
	}

	ranges := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), ",")
	parsedRanges := make([]httpRange, 0, len(ranges))

	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}

		i := strings.Index(r, "-")
		if i < 0 {
			return nil, fmt.Errorf("invalid range format")
		}

		start, end := strings.TrimSpace(r[:i]), strings.TrimSpace(r[i+1:])

		var startByte, endByte int64

		if start == "" {
			n, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid range format")
			}

			if n > size {
				n = size
			}

			startByte = size - n
			endByte = size - 1
		} else {
			n, err := strconv.ParseInt(start, 10, 64)
			if err != nil || n >= size {
				return nil, fmt.Errorf("invalid range format")
			}

			startByte = n
			if end == "" {
				endByte = size - 1
			} else {
				n, err := strconv.ParseInt(end, 10, 64)
				if err != nil || n >= size {
					endByte = size - 1
				} else {
					endByte = n
				}
			}
		}

		if startByte > endByte {
			return nil, fmt.Errorf("invalid range format")
		}

		parsedRanges = append(parsedRanges, httpRange{startByte, endByte})
	}

	return parsedRanges, nil
}
