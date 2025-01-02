package helpers

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type TranscodeOptions struct {
	Bin              string
	InputPath        string `json:"inputPath"`
	OutputDir        string
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

func validateVideoTranscodeOpts(opts *TranscodeOptions) error {
	if opts.Bin == "" {
		return fmt.Errorf("ffmpeg binary path is required")
	}

	if opts.InputPath == "" {
		return fmt.Errorf("input path is required")
	}

	if opts.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}

	_, err := os.Stat(opts.InputPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", opts.InputPath)
	}

	if opts.AudioStreamIndex < 0 {
		return fmt.Errorf("invalid audio stream index: %d", opts.AudioStreamIndex)
	}

	if opts.VideoStreamIndex < 0 {
		return fmt.Errorf("invalid video stream index: %d", opts.VideoStreamIndex)
	}

	if opts.AudioCodec != "copy" {
		if opts.AudioCodec == "" {
			return fmt.Errorf("audio codec is required when not copying")
		}

		if opts.AudioBitRate != "" && !strings.HasSuffix(opts.AudioBitRate, "k") {
			return fmt.Errorf("invalid audio bitrate format: %s (should end with 'k')", opts.AudioBitRate)
		}
	}

	if opts.VideoCodec != "copy" {
		if opts.VideoCodec == "" {
			return fmt.Errorf("video codec is required when not copying")
		}

		if opts.VideoBitrate != "" && !strings.HasSuffix(opts.VideoBitrate, "k") {
			return fmt.Errorf("invalid video bitrate format: %s (should end with 'k')", opts.VideoBitrate)
		}
	}

	if opts.VideoProfile != "" {
		validProfiles := []string{"baseline", "main", "high"}
		isValidProfile := false

		for _, p := range validProfiles {
			if opts.VideoProfile == p {
				isValidProfile = true
				break
			}
		}

		if !isValidProfile {
			return fmt.Errorf("invalid video profile: %s (must be one of: %s)", opts.VideoProfile, strings.Join(validProfiles, ", "))
		}
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
			return fmt.Errorf("invalid preset: %s (must be one of: %s)", opts.Preset, strings.Join(validPresets, ", "))
		}
	}

	return nil
}

func CreateHlsStream(opts *TranscodeOptions) error {
	err := validateVideoTranscodeOpts(opts)
	if err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	cmdArgs := []string{
		"-i", opts.InputPath,
		"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex),
		"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex),
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(opts.OutputDir, "segment_%03d.ts"),
		"-hls_playlist_type", "vod",
		"-progress", "pipe:1", // Output progress to stdout
	}

	if opts.AudioCodec == "copy" && opts.VideoCodec == "copy" {
		cmdArgs = append(cmdArgs, "-c", "copy")
	} else {
		if opts.AudioBitRate == "" {
			opts.AudioBitRate = "128k"
		}

		if opts.AudioChannels == "" {
			opts.AudioChannels = "2"
		}

		cmdArgs = append(cmdArgs,
			"-c:a", opts.AudioCodec,
			"-b:a", opts.AudioBitRate,
			"-ac", opts.AudioChannels,
		)

		if opts.VideoBitrate == "" {
			opts.VideoBitrate = "1000k"
		}

		if opts.VideoHeight == "" {
			opts.VideoHeight = "720"
		}

		if opts.VideoProfile == "" {
			opts.VideoProfile = "main"
		}

		if opts.Preset == "" {
			opts.Preset = "fast"
		}

		cmdArgs = append(cmdArgs,
			"-c:v", opts.VideoCodec,
			"-b:v", opts.VideoBitrate,
			"-vf", fmt.Sprintf("scale=w=-2:h=%s", opts.VideoHeight),
			"-preset", opts.Preset,
			"-profile:v", opts.VideoProfile,
			"-sc_threshold", "0", // Disable scene change detection
			"-g", "48", // Keyframe interval
			"-keyint_min", "48", // Minimum keyframe interval
		)
	}

	cmdArgs = append(cmdArgs, filepath.Join(opts.OutputDir, "playlist.m3u8"))

	cmd := exec.Command(opts.Bin, cmdArgs...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	errChan := make(chan error, 1)

	go func() {
		errOutput, err := io.ReadAll(stderr)
		if err != nil {
			errChan <- fmt.Errorf("failed to read stderr: %w", err)
			return
		}

		if len(errOutput) > 0 {
			errChan <- fmt.Errorf("ffmpeg error: %s", string(errOutput))
			return
		}

		errChan <- nil
	}()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	err = <-errChan
	if err != nil {
		return err
	}

	return nil
}
