package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"igloo/cmd/models"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/gofiber/fiber/v2"
)

var ffmpeg string = "/bin/ffmpeg"

func (h *Handlers) TranscodeVideo(c *fiber.Ctx) error {
	t := models.TranscodeVideoOpts{}

	err := c.BodyParser(&t)
	if err != nil {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	err = t.ValidateValues()
	if err != nil {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	err = os.MkdirAll(t.OutputDir, os.ModePerm)
	if err != nil {
		c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	args := []string{
		"-y",
		"-hwaccel", "cuda",
		"-i", t.FilePath,
		"-filter_complex", "[0:v]split=3[1080p_in][720p_in][480p_in];[1080p_in]scale=-2:1080[1080p_out];[720p_in]scale=-2:720[720p_out];[480p_in]scale=-2:480[480p_out]",
		"-map", "[1080p_out]", "-map", "[720p_out]", "-map", "[480p_out]",
		"-map", "0:a", "-map", "0:a", "-map", "0:a",
		"-c:v", "h264_nvenc", "-preset", "p4", "-cq", "23",
		"-pix_fmt", "yuv420p", // Add this line to ensure 8-bit output
		"-c:a", "aac", "-b:a", "192k",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-b:v:0", "6000k", "-maxrate:v:0", "6000k", "-bufsize:v:0", "8000k",
		"-b:v:1", "3500k", "-maxrate:v:1", "3500k", "-bufsize:v:1", "5000k",
		"-b:v:2", "1800k", "-maxrate:v:2", "1800k", "-bufsize:v:2", "3000k",
		"-var_stream_map", "v:0,a:0,name:1080p v:1,a:1,name:720p v:2,a:2,name:480p",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(t.OutputDir, fmt.Sprintf("%s_%%v_%%03d.ts", t.OutputName)),
		"-master_pl_name", fmt.Sprintf("%s_master.m3u8", t.OutputName),
		"-hls_flags", "independent_segments",
		"-hls_playlist_type", "event",
		filepath.Join(t.OutputDir, fmt.Sprintf("%s_%%v.m3u8", t.OutputName)),
	}

	cmd := exec.Command(ffmpeg, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pid := cmd.Process.Pid

	err = cmd.Start()
	if err != nil {
		c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"PID": pid,
	})
}

func (h *Handlers) CancelFfmpegProcess(c *fiber.Ctx) error {
	pidStr := c.Params("pid")
	if pidStr == "" {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Error": "pidis required",
		})

		return errors.New("pid is required")
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"Error": err.Error(),
		})

		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": fmt.Sprintf("Process with PID %d has been terminated", pid),
	})
}
