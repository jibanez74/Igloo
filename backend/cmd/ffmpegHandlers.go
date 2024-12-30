package main

import (
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"os"

	"github.com/gofiber/fiber/v2"
)

func (app *config) TranscodeMovieHls(c *fiber.Ctx) error {
	var req struct {
		MovieID    uint   `json:"movieID"`
		VideoCodec string `json:"videoCodec"`
		AudioCodec string `json:"audioCodec"`
	}

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var movie models.Movie
	movie.ID = req.MovieID

	status, err := app.repo.GetMovieByID(&movie)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	_, err = os.Stat(movie.FilePath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "movie not found",
		})
	}

	opts := helpers.TranscodeOptions{
		Bin:        app.ffmpeg,
		InputPath:  movie.FilePath,
		VideoCodec: req.VideoCodec,
	}

}
