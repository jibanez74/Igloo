package main

import (
	"fmt"
	"igloo/cmd/database/models"
	"igloo/cmd/helpers"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type transcodeRequest struct {
	AudioCodec     string `json:"audioCodec"`
	AudioBitRate   string `json:"audioBitRate"`
	AudioChannels  string `json:"audioChannels"`
	VideoCodec     string `json:"videoCodec"`
	VideoBitrate   string `json:"videoBitrate"`
	VideoHeight    string `json:"videoHeight"`
	VideoProfile   string `json:"videoProfile"`
	Preset         string `json:"preset"`
	CopyAudio      bool   `json:"copyAudio"`
	CopyVideoCodec bool   `json:"copyVideoCodec"`
}

func (app *config) TranscodeMovieHls(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid movie ID",
		})
	}

	var req transcodeRequest

	err = c.BodyParser(&req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	var movie models.Movie
	movie.ID = uint(id)

	status, err := app.repo.GetMovieByID(&movie)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": "movie not found",
		})
	}

	outputDir := filepath.Join(app.transcodeDir, fmt.Sprintf("%d", movie.ID))

	opts := &helpers.TranscodeOptions{
		Bin:              app.ffmpeg,
		InputPath:        movie.FilePath,
		OutputDir:        outputDir,
		AudioStreamIndex: 0, // Using first audio stream
		VideoStreamIndex: 0, // Using first video stream
		AudioBitRate:     req.AudioBitRate,
		AudioChannels:    req.AudioChannels,
		VideoBitrate:     req.VideoBitrate,
		VideoHeight:      req.VideoHeight,
		VideoProfile:     req.VideoProfile,
		Preset:           req.Preset,
	}

	if req.CopyAudio {
		opts.AudioCodec = "copy"
	} else {
		opts.AudioCodec = req.AudioCodec
	}

	if req.CopyVideoCodec {
		opts.VideoCodec = "copy"
	} else {
		opts.VideoCodec = req.VideoCodec
	}

	err = helpers.CreateHlsStream(opts)
	if err != nil {
		os.RemoveAll(outputDir)

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to create HLS stream: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"fileName": filepath.Base(outputDir),
	})
}
