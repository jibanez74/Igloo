package main

import (
	"fmt"
	"igloo/cmd/internal/ffmpeg"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *application) createMovieHlsStream(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to parse id %s", c.Params("id")),
		})
	}

	movie, err := app.queries.GetMovieForStreaming(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "movie not found",
		})
	}

	var request ffmpeg.HlsOpts

	err = c.BodyParser(&request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse hls options",
		})
	}

	request.InputPath = movie.FilePath
	request.OutputDir = filepath.Join(
		app.settings.GetTranscodeDir(),
		"movies",
		fmt.Sprintf("%d", movie.ID),
	)
	request.SegmentsUrl = fmt.Sprintf("/api/v1/hls/movies/%d", movie.ID)

	pid, err := app.ffmpeg.CreateHlsStream(&request)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"pid": pid,
	})
}

func (app *application) cancelJob(c *fiber.Ctx) error {
	pid := c.Params("pid")
	if pid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pid is required",
		})
	}

	err := app.ffmpeg.CancelJob(pid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "job cancelled",
	})
}
