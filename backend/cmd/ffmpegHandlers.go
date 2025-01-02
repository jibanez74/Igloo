package main

import (
	"fmt"
	"igloo/cmd/helpers"
	"log"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

func (app *config) TranscodeHls(c *fiber.Ctx) error {
	var req helpers.TranscodeOptions

	err := c.BodyParser(&req)
	if err != nil {
		log.Println(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	req.Bin = app.ffmpeg
	req.OutputDir = filepath.Join(app.transcodeDir, filepath.Base(req.InputPath))

	err = helpers.CreateHlsStream(&req)
	if err != nil {
		log.Println(err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to create HLS stream: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"fileName": filepath.Base(req.OutputDir),
		"message":  "HLS stream created successfully",
	})
}
