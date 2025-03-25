package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffmpeg"
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

func (app *application) createMovieHls(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse id from url params",
		})
	}

	var req ffmpeg.HlsOpts

	err = c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse request body for hls opts",
		})
	}

	movie, err := app.queries.GetMovieForHls(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	req.InputPath = movie.FilePath
	req.OutputDir = fmt.Sprintf("%s/movies/%d", app.settings.TranscodeDir, movie.ID)
	req.SegmentsUrl = fmt.Sprintf("/api/v1/static/movies/%d/", movie.ID)

	pid, err := app.ffmpeg.CreateHlsStream(&req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"pid":      pid,
		"m3u8_url": fmt.Sprintf("/api/v1/transcode/movies/%d/%s", movie.ID, ffmpeg.DefaultPlaylistName),
	})
}
