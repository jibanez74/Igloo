package main

import (
	"fmt"
	"igloo/cmd/helpers"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

func (app *config) TranscodeHls(c *fiber.Ctx) error {
	var req helpers.TranscodeOptions

	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	req.Bin = app.ffmpeg
	req.OutputDir = filepath.Join(app.transcodeDir, filepath.Base(req.InputPath))

	err = helpers.CreateHlsStream(&req)
	if err != nil {
		os.RemoveAll(req.OutputDir)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to create HLS stream: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"fileName": filepath.Base(req.OutputDir),
		"message":  "HLS stream created successfully",
	})
}
