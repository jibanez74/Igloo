package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"igloo/cmd/internal/helpers"
)

const (
	hlsStderrScannerBufferSize = 64 * 1024
	hlsStderrScannerMaxToken   = 1024 * 1024
)

// HLSParams holds inputs for HLS transcoding: argument building and RunHLS.
//
// VideoStreamIndex and AudioStreamIndex are global ffprobe stream indices
// used directly with -map 0:<index>.
type HLSParams struct {
	SourcePath       string
	OutDir           string
	Profile          string
	VideoStreamIndex int
	AudioStreamIndex int
	HWDevice         string
	CopyVideo        bool
	CopyAudio        bool
	StartSec         float64
}

// hlsHWTranscode maps hardware acceleration device IDs to FFmpeg -hwaccel and
// -c:v encoder names. CPU and unknown devices fall back to libx264 (no -hwaccel).
var hlsHWTranscodeByDevice = map[string]struct {
	HWAccel string
	Encoder string
}{
	helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:  {HWAccel: "videotoolbox", Encoder: "h264_videotoolbox"},
	helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA: {HWAccel: "cuda", Encoder: "h264_nvenc"},
	helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:  {HWAccel: "qsv", Encoder: "h264_qsv"},
}

func buildHLSArgs(p HLSParams) ([]string, error) {
	if !helpers.IsAllowedHLSProfile(p.Profile) {
		return nil, fmt.Errorf("invalid HLS profile: %s", p.Profile)
	}

	copyVideo := p.CopyVideo
	if p.Profile == helpers.HLS_PROFILE_REMUX {
		copyVideo = true
	}

	var cfg helpers.HLSProfileConfig
	if !copyVideo {
		var ok bool
		cfg, ok = helpers.HLSProfileConfigs[p.Profile]
		if !ok {
			return nil, fmt.Errorf("unknown HLS profile: %s", p.Profile)
		}
	}

	args := []string{"-y", "-fflags", "+genpts", "-analyzeduration", "20000000", "-probesize", "20000000"}

	hwLower := strings.ToLower(p.HWDevice)
	hw, hwKnown := hlsHWTranscodeByDevice[hwLower]

	if !copyVideo && hwKnown && hw.HWAccel != "" {
		args = append(args, "-hwaccel", hw.HWAccel)
	}

	if p.StartSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", p.StartSec))
	}

	args = append(args,
		"-i", p.SourcePath,
		"-map", fmt.Sprintf("0:%d", p.VideoStreamIndex),
		"-map", fmt.Sprintf("0:%d", p.AudioStreamIndex),
	)

	if copyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		if hwKnown {
			args = append(args, "-c:v", hw.Encoder)
		} else {
			args = append(args, "-c:v", "libx264", "-preset", "veryfast")
		}
		args = append(args,
			"-b:v", cfg.VideoBitrate,
			"-maxrate", cfg.VideoBitrate,
			"-bufsize", cfg.Bufsize,
			"-vf", fmt.Sprintf("scale=-2:%d", cfg.Height),
		)
	}

	if p.CopyAudio {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
	}

	args = append(args, "-avoid_negative_ts", "make_zero", "-max_muxing_queue_size", "1024")

	segmentPattern := filepath.Join(p.OutDir, "segment_%d.m4s")
	args = append(args,
		"-f", "hls",
		"-hls_segment_type", "fmp4",
		"-hls_playlist_type", "event",
		"-hls_list_size", "0",
		"-hls_time", fmt.Sprintf("%d", helpers.HLS_SEGMENT_TIME_SEC),
		"-hls_segment_filename", segmentPattern,
		"-hls_fmp4_init_filename", "init.mp4",
		filepath.Join(p.OutDir, "playlist.m3u8"),
	)

	return args, nil
}

// RunHLS starts FFmpeg in the background for HLS transcoding.
// It does not block on process exit. onExit is called asynchronously when
// the process finishes; stderrTail contains the last HLS_STDERR_TAIL_LINES
// of FFmpeg's stderr output for error diagnostics. The returned command is
// for process lifecycle control only; RunHLS owns cmd.Wait and exit delivery.
func (f *ffmpeg) RunHLS(
	ctx context.Context,
	params HLSParams,
	onExit func(exitErr error, stderrTail []string),
) (*exec.Cmd, error) {
	outDir := strings.TrimSpace(params.OutDir)
	if outDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	absOutDir, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory: %w", err)
	}

	info, err := os.Stat(absOutDir)
	if err != nil {
		return nil, fmt.Errorf("stat output directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("output path is not a directory: %s", absOutDir)
	}

	params.OutDir = absOutDir

	args, err := buildHLSArgs(params)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, f.bin, args...)
	cmd.Dir = absOutDir
	cmd.Stdout = nil

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	ring := make([]string, helpers.HLS_STDERR_TAIL_LINES)
	var ringIdx int
	var ringCount int
	appendTail := func(line string) {
		ring[ringIdx%helpers.HLS_STDERR_TAIL_LINES] = line
		ringIdx++
		ringCount++
	}

	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	go func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, hlsStderrScannerBufferSize), hlsStderrScannerMaxToken)
		for scanner.Scan() {
			appendTail(scanner.Text())
		}
		if scanErr := scanner.Err(); scanErr != nil {
			appendTail(fmt.Sprintf("stderr scan error: %v", scanErr))
			_, _ = io.Copy(io.Discard, stderrPipe)
		}
	}()

	startErr := cmd.Start()
	if startErr != nil {
		return nil, startErr
	}

	go func() {
		exitErr := cmd.Wait()
		stderrWg.Wait()
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
