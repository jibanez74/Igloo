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
	request.OutputDir = filepath.Join(app.settings.GetTranscodeDir(), "movies", fmt.Sprintf("%d", movie.ID))

	err = app.ffmpeg.CreateHlsStream(&request)
	if err != nil {
		msg := fmt.Sprintf("an error occurred while creating the hls stream: %s", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": msg,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "hls stream created successfully",
	})
}
