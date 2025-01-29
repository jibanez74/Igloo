package main

import (
	"igloo/cmd/internal/database/models"
	"igloo/cmd/internal/ffmpeg"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

type hlsRequestOpts struct {
	MovieID    uint   `json:"movieID"`
	VideoCodec string `json:"videoCodec"`
	AudioCodec string `json:"audioCodec"`
}

func (app *config) createMovieHls(c *fiber.Ctx) error {
	var req hlsRequestOpts

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var movie models.Movie

	err = app.db.First(&movie, req.MovieID).Error
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	opts := ffmpeg.HlsOpts{
		InputPath:        movie.FilePath,
		OutputDir:        filepath.Join(app.settings.TranscodeDir, "movies", movie.Title),
		VideoCodec:       req.VideoCodec,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		AudioCodec:       req.AudioCodec,
	}

	err = app.ffmpeg.CreateHlsStream(&opts)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"opts": opts,
	})
}
