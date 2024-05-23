package handlers

import (
	"igloo/helpers"
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *AppHandlers) DirectPlayVideo(c *fiber.Ctx) error {
	movieId, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid movie id",
		})
	}

	var movie struct {
		FilePath string
	}

	err = h.db.Model(&models.Movie{}).Select("file_path").First(&movie, uint(movieId)).Error
	if err != nil {
		return c.Status(helpers.GetStatusCode(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// set the necesary headers
	c.Set("Content-Type", "video/mp4")
	c.Set("Content-Disposition", "inline")
	c.Set("Accept-Ranges", "bytes")

	return c.SendFile("/Users/romany/Movies" + movie.FilePath)
}
