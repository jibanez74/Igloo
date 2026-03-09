package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"igloo/cmd/internal/helpers"
)

// BuildHLSArgs builds FFmpeg arguments for HLS fMP4 output.
//
// Arg ordering follows FFmpeg requirements:
//
//	[global] [-hwaccel …] [-i source] [-map …] [-c:v … -c:a …] [-f hls …] output
//
// videoStreamIndex / audioStreamIndex are 0-based indices among streams of that type
// (i.e. 0 = first video, 0 = first audio).
// copyVideo / copyAudio control codec copy independently — e.g. video may need
// transcoding while AAC audio can be passed through without re-encoding.
func BuildHLSArgs(
	sourcePath, outDir, profile string,
	videoStreamIndex, audioStreamIndex int,
	hwDevice string,
	copyVideo, copyAudio bool,
) ([]string, error) {
	if !helpers.IsAllowedHLSProfile(profile) {
		return nil, fmt.Errorf("invalid HLS profile: %s", profile)
	}
	cfg, ok := helpers.HLSProfileConfigs[profile]
	if !ok {
		return nil, fmt.Errorf("unknown HLS profile: %s", profile)
	}

	// --- global + input ---
	args := []string{"-y", "-fflags", "+genpts", "-analyzeduration", "20000000", "-probesize", "20000000"}

	// -hwaccel MUST come before -i
	if !copyVideo {
		switch strings.ToLower(hwDevice) {
		case helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
			args = append(args, "-hwaccel", "videotoolbox")
		case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
			args = append(args, "-hwaccel", "cuda")
		case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
			args = append(args, "-hwaccel", "qsv")
		}
	}

	args = append(args,
		"-i", sourcePath,
		"-map", fmt.Sprintf("0:v:%d", videoStreamIndex),
		"-map", fmt.Sprintf("0:a:%d", audioStreamIndex),
	)

	// --- video encoding ---
	if copyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		switch strings.ToLower(hwDevice) {
		case helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
			args = append(args, "-c:v", "h264_videotoolbox")
		case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
			args = append(args, "-c:v", "h264_nvenc")
		case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
			args = append(args, "-c:v", "h264_qsv")
		default:
			args = append(args, "-c:v", "libx264", "-preset", "veryfast")
		}
		args = append(args,
			"-b:v", cfg.VideoBitrate,
			"-maxrate", cfg.VideoBitrate,
			"-bufsize", cfg.Bufsize,
			"-vf", fmt.Sprintf("scale=-2:%d", cfg.Height),
		)
	}

	// --- audio encoding ---
	if copyAudio {
		args = append(args, "-c:a", "copy")
	} else {
		// Downmix to stereo: browser MSE decoders generally don't support
		// multichannel AAC (PCE-encoded 5.1/7.1). Stereo at 192k is
		// universally supported and sounds good for web playback.
		args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
	}

	args = append(args, "-avoid_negative_ts", "make_zero", "-max_muxing_queue_size", "1024")

	// --- HLS output ---
	segmentPattern := filepath.Join(outDir, "segment_%d.m4s")
	args = append(args,
		"-f", "hls",
		"-hls_segment_type", "fmp4",
		"-hls_playlist_type", "event",
		"-hls_list_size", "0",
		"-hls_time", fmt.Sprintf("%d", helpers.HLS_SEGMENT_TIME_SEC),
		"-hls_segment_filename", segmentPattern,
		"-hls_fmp4_init_filename", "init.mp4",
		filepath.Join(outDir, "playlist.m3u8"),
	)

	return args, nil
}

// RunHLS starts FFmpeg in the background for HLS transcoding.
// It does not block on process exit. onExit is called asynchronously when
// the process finishes; stderrTail contains the last HLS_STDERR_TAIL_LINES
// of FFmpeg's stderr output for error diagnostics.
func (f *FFmpeg) RunHLS(
	ctx context.Context,
	sourcePath, outDir, profile string,
	videoStreamIndex, audioStreamIndex int,
	hwDevice string,
	copyVideo, copyAudio bool,
	onExit func(exitErr error, stderrTail []string),
) (*exec.Cmd, error) {
	args, err := BuildHLSArgs(sourcePath, outDir, profile, videoStreamIndex, audioStreamIndex, hwDevice, copyVideo, copyAudio)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, f.bin, args...)
	cmd.Dir = outDir
	cmd.Stdout = nil

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	// Ring buffer: keep only the last HLS_STDERR_TAIL_LINES for error reporting.
	ring := make([]string, helpers.HLS_STDERR_TAIL_LINES)
	var ringIdx int
	var ringCount int

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			ring[ringIdx%helpers.HLS_STDERR_TAIL_LINES] = scanner.Text()
			ringIdx++
			ringCount++
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		exitErr := cmd.Wait()
		if onExit != nil {
			n := helpers.HLS_STDERR_TAIL_LINES
			if ringCount < n {
				n = ringCount
			}
			tail := make([]string, 0, n)
			start := ringIdx - n
			if start < 0 {
				start = 0
			}
			for i := start; i < ringIdx; i++ {
				tail = append(tail, ring[i%helpers.HLS_STDERR_TAIL_LINES])
			}
			onExit(exitErr, tail)
		}
	}()

	return cmd, nil
}
