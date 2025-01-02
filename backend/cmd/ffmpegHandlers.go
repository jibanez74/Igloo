package main

import (
	"fmt"
	"igloo/cmd/helpers"
	"log"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

func (app *config) TranscodeHls(c *fiber.Ctx) error {
	var req struct {
		InputPath        string `json:"inputPath"`
		AudioStreamIndex int    `json:"audioStreamIndex"`
		AudioCodec       string `json:"audioCodec"`
		AudioBitRate     string `json:"audioBitRate"`
		AudioChannels    string `json:"audioChannels"`
		VideoStreamIndex int    `json:"videoStreamIndex"`
		VideoCodec       string `json:"videoCodec"`
		VideoBitrate     string `json:"videoBitrate"`
		VideoHeight      string `json:"videoHeight"`
		VideoProfile     string `json:"videoProfile"`
		Preset           string `json:"preset"`
	}

	err := c.BodyParser(&req)
	if err != nil {
		log.Println(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	opts := helpers.TranscodeOptions{
		Bin:              app.ffmpeg,
		InputPath:        req.InputPath,
		OutputDir:        filepath.Join(app.transcodeDir, filepath.Base(req.InputPath)),
		AudioStreamIndex: req.AudioStreamIndex,
		AudioCodec:       req.AudioCodec,
		AudioBitRate:     req.AudioBitRate,
		AudioChannels:    req.AudioChannels,
		VideoStreamIndex: req.VideoStreamIndex,
		VideoCodec:       req.VideoCodec,
		VideoBitrate:     req.VideoBitrate,
		VideoHeight:      req.VideoHeight,
		VideoProfile:     req.VideoProfile,
		Preset:           req.Preset,
	}

	status, err := helpers.ValidateVideoTranscodeOpts(&opts)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	err = helpers.CreateHlsStream(&opts)
	if err != nil {
		log.Println(err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to create HLS stream: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"fileName": filepath.Base(opts.OutputDir),
		"message":  "HLS stream created successfully",
	})
}
