package main

import (
	"fmt"
	"igloo/cmd/internal/ffmpeg"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

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
	req.StartTime = 0

	log.Println("about to start command")

	pid, err := app.ffmpeg.CreateHlsStream(&req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	log.Println("about to return response")

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"pid":      pid,
		"m3u8_url": fmt.Sprintf("/api/v1/transcode/movies/%d/%s", movie.ID, ffmpeg.DefaultPlaylistName),
	})
}

func (app *application) cancelFFmpegJob(c *fiber.Ctx) error {
	pid := c.Params("pid")
	if pid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "job pid is required",
		})
	}

	err := app.ffmpeg.CancelJob(pid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to cancel job: %v", err),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "job cancelled successfully",
	})
}
