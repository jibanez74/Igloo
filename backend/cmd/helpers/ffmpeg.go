package helpers

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type TranscodeOptions struct {
	Bin              string
	InputPath        string
	OutputDir        string
	AudioStreamIndex int
	AudioCodec       string
	AudioBitRate     string
	AudioChannels    string
	VideoStreamIndex int
	VideoCodec       string
	VideoBitrate     string
	VideoHeight      string
	VideoProfile     string
	Preset           string
}

func ValidateVideoTranscodeOpts(opts *TranscodeOptions) (int, error) {
	if opts.Bin == "" {
		return fiber.StatusInternalServerError, fmt.Errorf("ffmpeg binary path is required")
	}

	if opts.OutputDir == "" {
		return fiber.StatusInternalServerError, fmt.Errorf("output directory is required")
	}

	if opts.InputPath == "" {
		return fiber.StatusBadRequest, fmt.Errorf("input path is required")
	}

	status, err := CheckFileExist(opts.InputPath)
	if err != nil {
		return status, err
	}

	if opts.AudioStreamIndex < 0 {
		return fiber.StatusBadRequest, fmt.Errorf("invalid audio stream index: %d", opts.AudioStreamIndex)
	}

	if opts.VideoStreamIndex < 0 {
		return fiber.StatusBadRequest, fmt.Errorf("invalid video stream index: %d", opts.VideoStreamIndex)
	}

	if opts.Preset != "" {
		validPresets := []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}
		isValidPreset := false

		for _, p := range validPresets {
			if opts.Preset == p {
				isValidPreset = true
				break
			}
		}

		if !isValidPreset {
			return fiber.StatusBadRequest, fmt.Errorf("invalid preset: %s (must be one of: %s)",
				opts.Preset, strings.Join(validPresets, ", "))
		}
	}

	return 0, nil
}

func CreateHlsStream(opts *TranscodeOptions) error {
	cmdArgs := []string{
		"-i", opts.InputPath,
		"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex), // Video stream
		"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex), // Audio stream
	}

	if opts.AudioCodec == "copy" && opts.VideoCodec == "copy" {
		cmdArgs = append(cmdArgs, "-c:v", "copy", "-c:a", "copy")
	} else {
		if opts.AudioCodec == "copy" {
			cmdArgs = append(cmdArgs, "-c:a", "copy")
		} else {
			cmdArgs = append(cmdArgs,
				"-c:a", opts.AudioCodec,
				"-b:a", opts.AudioBitRate,
				"-ac", opts.AudioChannels,
			)
		}

		if opts.VideoCodec == "copy" {
			cmdArgs = append(cmdArgs, "-c:v", "copy")
		} else {
			cmdArgs = append(cmdArgs,
				"-c:v", opts.VideoCodec,
				"-b:v", opts.VideoBitrate,
				"-vf", fmt.Sprintf("scale=-2:%s", opts.VideoHeight),
				"-preset", opts.Preset,
				"-profile:v", opts.VideoProfile,
			)
		}
	}

	cmdArgs = append(cmdArgs,
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(opts.OutputDir, "segment_%03d.ts"),
		"-hls_playlist_type", "vod",
	)

	cmdArgs = append(cmdArgs, filepath.Join(opts.OutputDir, "playlist.m3u8"))

	cmd := exec.Command(opts.Bin, cmdArgs...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	errOutput, _ := io.ReadAll(stderr)

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(errOutput))
	}

	return nil
}
