package main

import (
	"fmt"
	"igloo/cmd/internal/ffmpeg"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *application) createHlsStream(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse movie id from url params",
		})
	}

	var req ffmpeg.HlsOpts

	err = c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unable to parse hls options from request body",
		})
	}

	movie, err := app.queries.GetMovieForHls(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to fetch movie with id %d", id),
		})
	}

	dirName := fmt.Sprintf("movie_%d", movie.ID)

	req.InputPath = movie.FilePath
	req.OutputDir = filepath.Join(app.settings.TranscodeDir, "movies", dirName)

	pid, err := app.ffmpeg.CreateHlsStream(&req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "unable to transcode using ffmpeg",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"pid":      pid,
		"m3u8_url": fmt.Sprintf("/api/v1/transcode/movies/%s/%s", dirName, ffmpeg.DefaultPlaylistName),
	})
}
