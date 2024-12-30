package helpers

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

func CreateHlsStream(opts TranscodeOptions) error {
	// Base command with input and stream selection
	cmdArgs := []string{
		"-i", opts.InputPath,
		// Map only the specified video and audio streams
		"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex), // Video stream
		"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex), // Audio stream
		"-f", "hls",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(opts.OutputDir, "segment_%03d.ts"),
		"-hls_playlist_type", "vod",
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
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
