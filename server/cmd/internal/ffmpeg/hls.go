package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"igloo/cmd/internal/helpers"
)

type RunHLSConfig struct {
	Ctx           context.Context
	Log           *slog.Logger
	OnExit        func(err error)
	SourcePath    string
	OutDir        string
	Profile       string
	AudioTrackIdx int
	VideoTrackIdx int
	HWDevice      string
	UseFastPath   bool
}

// RunHLS starts FFmpeg in the background to produce HLS fMP4 output in cfg.OutDir.
// It does not block: it starts the process and a goroutine that reads stderr and calls Wait().
// When the process exits, cfg.OnExit is called with the exit error (nil if success).
// The caller must kill the returned Cmd on cleanup and delete cfg.OutDir.
func (f *ffmpeg) RunHLS(cfg *RunHLSConfig) (*exec.Cmd, error) {
	args, err := buildHLSArgs(cfg.SourcePath, cfg.OutDir, cfg.Profile, cfg.AudioTrackIdx, cfg.VideoTrackIdx, cfg.HWDevice, cfg.UseFastPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(cfg.Ctx, f.bin, args...)
	cmd.Dir = cfg.OutDir
	cmd.Stdin = nil

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	go func() {
		defer func() {
			exitErr := cmd.Wait()
			if cfg.OnExit != nil {
				cfg.OnExit(exitErr)
			}
		}()

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				cfg.Log.Debug("ffmpeg", "stderr", line)
			}
		}
	}()

	return cmd, nil
}

// buildHLSArgs returns FFmpeg arguments for HLS fMP4. Paths are used as single arguments (no injection).
func buildHLSArgs(
	sourcePath, outDir, profile string,
	audioTrackIdx, videoTrackIdx int,
	hwDevice string,
	useFastPath bool,
) ([]string, error) {
	if !helpers.IsAllowedHLSProfile(profile) {
		return nil, fmt.Errorf("invalid HLS profile %q", profile)
	}

	cfg, ok := helpers.HLSProfileConfigs[profile]
	if !ok {
		return nil, fmt.Errorf("no config for HLS profile %q", profile)
	}

	// Input: reliable opening of large files.
	args := []string{
		"-analyzeduration", "20000000",
		"-probesize", "20000000",
		"-i", sourcePath,
	}

	// Video and audio map. Caller passes validated indices (Phase 4 validates).
	args = append(args, "-map", fmt.Sprintf("0:v:%d", videoTrackIdx))
	args = append(args, "-map", fmt.Sprintf("0:a:%d", audioTrackIdx))

	// Video codec: HW or libx264, or copy for fast path.
	if useFastPath {
		args = append(args, "-c:v", "copy", "-c:a", "copy")
	} else {
		hwAccel, encoder := hwDeviceToFFmpeg(hwDevice)
		if hwAccel != "" {
			args = append(args, "-hwaccel", hwAccel)
		}

		args = append(args, "-c:v", encoder)
		args = append(args, "-vf", cfg.ScaleFilter)
		args = append(args, "-b:v", cfg.VideoBitrate)
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}

	// HLS fMP4 output (main plan §5.11).
	playlistPath := filepath.Join(outDir, "playlist.m3u8")
	segmentPattern := filepath.Join(outDir, "segment_%d.m4s")
	initPath := filepath.Join(outDir, "init.mp4")

	args = append(args,
		"-f", "hls",
		"-hls_segment_type", "fmp4",
		"-hls_playlist_type", helpers.HLS_PLAYLIST_VOD,
		"-hls_list_size", helpers.HLS_PLAYLIST_SIZE,
		"-hls_time", fmt.Sprintf("%d", helpers.HLSSegmentTimeSeconds),
		"-hls_segment_filename", segmentPattern,
		"-hls_fmp4_init_filename", initPath,
		playlistPath,
	)

	return args, nil
}

// hwDeviceToFFmpeg returns (-hwaccel value, -c:v encoder) for the given device.
// Empty hwAccel means no hwaccel (use with libx264).
func hwDeviceToFFmpeg(hwDevice string) (hwAccel, encoder string) {
	switch hwDevice {
	case helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
		return "videotoolbox", "h264_videotoolbox"
	case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
		return "cuda", "h264_nvenc"
	case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
		return "qsv", "h264_qsv"
	default:
		return "", "libx264"
	}
}

// BuildHLSArgsForTest is used by tests to assert on built args without running FFmpeg.
// It returns the same slice that RunHLS would pass to exec, or an error.
func BuildHLSArgsForTest(
	sourcePath, outDir, profile string,
	audioTrackIdx, videoTrackIdx int,
	hwDevice string,
	useFastPath bool,
) ([]string, error) {
	return buildHLSArgs(sourcePath, outDir, profile, audioTrackIdx, videoTrackIdx, hwDevice, useFastPath)
}
