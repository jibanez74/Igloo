package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"igloo/cmd/internal/helpers"
)

const hlsIntelQSVDeviceName = "igloo_qsv"

func isExpectedHLSStderrClose(err error) bool {
	return errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe)
}

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
	TonemapHDR       bool // true when source is HDR and the profile requires SDR output
	SourceFrameRate  float64
	Capabilities     Capabilities
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

	if strings.TrimSpace(p.SourcePath) == "" {
		return nil, fmt.Errorf("source path is required")
	}

	if p.VideoStreamIndex < 0 {
		return nil, fmt.Errorf("video stream index must be non-negative")
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

	// Thread count is intentionally left unset: libx264 (and the hardware
	// encoders) auto-detect an appropriate value, which lets a single transcode
	// use the whole machine. Total CPU pressure is bounded by the concurrency
	// limiter in the api package, not by a per-process thread cap.
	args := []string{
		"-y", "-fflags", "+genpts",
		"-analyzeduration", "5000000", "-probesize", "5000000",
	}

	deviceDecision := ResolveHLSDevice(p.HWDevice, p.Capabilities)
	hwLower := deviceDecision.Effective
	hw, hwKnown := hlsHWTranscodeByDevice[hwLower]
	useNvidiaCUDAFilters := !copyVideo && hwLower == helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA &&
		p.Capabilities.SupportsNvidiaCUDAFilters(p.TonemapHDR)
	useIntelQSVScale := !copyVideo &&
		hwLower == helpers.HARDWARE_ACCELERATION_DEVICE_INTEL &&
		!p.TonemapHDR &&
		p.Capabilities.SupportsIntelQSVScale()

	if !copyVideo {
		if useIntelQSVScale {
			args = append(args,
				"-init_hw_device", "qsv="+hlsIntelQSVDeviceName,
				"-filter_hw_device", hlsIntelQSVDeviceName,
			)
		}
		switch {
		case useNvidiaCUDAFilters:
			args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
		case hwKnown && hw.HWAccel != "" && hwLower == helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
			args = append(args, "-hwaccel", hw.HWAccel)
		}
	}

	if p.StartSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", p.StartSec))
	}

	args = append(args,
		"-i", p.SourcePath,
		"-map", fmt.Sprintf("0:%d", p.VideoStreamIndex),
	)
	hasAudio := p.AudioStreamIndex >= 0
	if hasAudio {
		args = append(args, "-map", fmt.Sprintf("0:%d", p.AudioStreamIndex))
	}
	args = append(args,
		"-map_metadata", "-1",
		"-map_chapters", "-1",
	)

	if copyVideo {
		args = append(args, "-c:v", "copy")
	} else {
		encoder := "libx264"
		if hwKnown {
			encoder = hw.Encoder
		}

		args = append(args, "-c:v", encoder, "-profile:v", "high")
		switch hwLower {
		case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
			args = append(args, "-rc", "vbr", "-preset", "p4")
		case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
			args = appendHLSIntelEncoderArgs(args, p.Capabilities)
		case helpers.HARDWARE_ACCELERATION_DEVICE_CPU:
			args = append(args, "-preset", "veryfast")
		}

		args = append(args,
			"-b:v", cfg.VideoBitrate,
			"-maxrate", cfg.VideoBitrate,
			"-bufsize", cfg.Bufsize,
		)

		args = append(args, "-vf", hlsVideoFilter(cfg, hwLower, p.TonemapHDR, useNvidiaCUDAFilters, useIntelQSVScale))
		if shouldSetHLSPixelFormat(encoder, useNvidiaCUDAFilters) {
			args = append(args, "-pix_fmt", "yuv420p")
		}
		args = append(args,
			"-color_primaries", "bt709",
			"-color_trc", "bt709",
			"-colorspace", "bt709",
		)
		args = appendHLSKeyframeArgs(args, encoder, p.SourceFrameRate)
	}

	if hasAudio {
		if p.CopyAudio {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "320k")
		}
	}

	args = append(args, "-avoid_negative_ts", "make_zero", "-max_muxing_queue_size", "1024")

	segmentPattern := filepath.Join(p.OutDir, "segment_%d.m4s")
	args = append(args,
		"-f", "hls",
		"-hls_segment_type", "fmp4",
		"-hls_segment_options", "movflags=+frag_discont",
		"-hls_playlist_type", "event",
		"-hls_list_size", "0",
		"-hls_time", fmt.Sprintf("%d", helpers.HLS_SEGMENT_TIME_SEC),
		"-hls_segment_filename", segmentPattern,
		"-hls_fmp4_init_filename", "init.mp4",
		filepath.Join(p.OutDir, "playlist.m3u8"),
	)

	return args, nil
}

func appendHLSIntelEncoderArgs(args []string, caps Capabilities) []string {
	if caps.SupportsEncoderOption("h264_qsv", "preset") {
		args = append(args, "-preset", "veryfast")
	}
	if caps.SupportsEncoderOption("h264_qsv", "look_ahead") {
		args = append(args, "-look_ahead", "1")
	}
	if caps.SupportsEncoderOption("h264_qsv", "forced_idr") {
		args = append(args, "-forced_idr", "1")
	}
	return args
}

func shouldSetHLSPixelFormat(encoder string, useNvidiaCUDAFilters bool) bool {
	if useNvidiaCUDAFilters {
		return false
	}
	return !strings.EqualFold(encoder, "h264_qsv")
}

func hlsSoftwareOutputPixelFormat(hwDevice string) string {
	if hwDevice == helpers.HARDWARE_ACCELERATION_DEVICE_INTEL {
		return "nv12"
	}
	return "yuv420p"
}

func hlsVideoFilter(
	cfg helpers.HLSProfileConfig,
	hwDevice string,
	tonemapHDR bool,
	useNvidiaCUDAFilters bool,
	useIntelQSVScale bool,
) string {
	switch {
	case tonemapHDR && useNvidiaCUDAFilters:
		return fmt.Sprintf(
			"scale_cuda=w=-2:h=%d:format=p010,"+
				"tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0",
			cfg.Height,
		)
	case useNvidiaCUDAFilters:
		return fmt.Sprintf("scale_cuda=w=-2:h=%d:format=yuv420p", cfg.Height)
	case useIntelQSVScale:
		return fmt.Sprintf(
			"format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=%d:format=nv12",
			cfg.Height,
		)
	case tonemapHDR && hwDevice == helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
		return fmt.Sprintf(
			"scale_vt=w=-2:h=%d:color_matrix=bt709:color_primaries=bt709:color_transfer=bt709",
			cfg.Height,
		)
	case tonemapHDR:
		outputFormat := hlsSoftwareOutputPixelFormat(hwDevice)
		return fmt.Sprintf(
			"zscale=w=-2:h=%d:t=linear:npl=100,format=gbrpf32le,"+
				"zscale=p=bt709,tonemap=tonemap=hable:desat=0,"+
				"zscale=t=bt709:m=bt709:r=tv,format=%s,%s",
			cfg.Height,
			outputFormat,
			helpers.HLS_SDR_COLOR_PARAMS,
		)
	default:
		outputFormat := hlsSoftwareOutputPixelFormat(hwDevice)
		return fmt.Sprintf("scale=-2:%d,format=%s,%s", cfg.Height, outputFormat, helpers.HLS_SDR_COLOR_PARAMS)
	}
}

func appendHLSKeyframeArgs(args []string, encoder string, frameRate float64) []string {
	segmentTime := float64(helpers.HLS_SEGMENT_TIME_SEC)
	isGOPDrivenEncoder := strings.EqualFold(encoder, "h264_nvenc") ||
		strings.EqualFold(encoder, "h264_qsv") ||
		strings.EqualFold(encoder, "h264_videotoolbox")

	if frameRate > 0 {
		gop := int(math.Ceil(segmentTime * frameRate))
		if gop > 0 {
			args = append(args,
				"-g:v:0", fmt.Sprintf("%d", gop),
				"-keyint_min:v:0", fmt.Sprintf("%d", gop),
			)
			if isGOPDrivenEncoder {
				return args
			}
		}
	}

	// For software encoders (and as a fallback when frame rate is unknown) we
	// force keyframes on segment boundaries via an expression. This is kept even
	// when -g/-keyint_min were set above: those make the GOP the right *size*,
	// while -force_key_frames is what actually pins keyframes to the exact
	// segment timestamps so every HLS segment starts on an IDR frame.
	// -sc_threshold:v:0 0 disables libx264 scene-cut keyframes so it does not
	// insert extra, unaligned IDR frames between the forced ones.
	args = append(args,
		"-force_key_frames:0",
		fmt.Sprintf("expr:gte(t,n_forced*%d)", helpers.HLS_SEGMENT_TIME_SEC),
	)
	if strings.EqualFold(encoder, "libx264") {
		args = append(args, "-sc_threshold:v:0", "0")
	}
	return args
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
	if !params.Capabilities.Probed {
		params.Capabilities = f.capabilities
	}

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
	appendTail := func(line string) {
		ring[ringIdx%helpers.HLS_STDERR_TAIL_LINES] = line
		ringIdx++
	}

	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	go func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, helpers.HLS_STDERR_SCANNER_BUFFER_SIZE), helpers.HLS_STDERR_SCANNER_MAX_TOKEN)
		for scanner.Scan() {
			appendTail(scanner.Text())
		}
		if scanErr := scanner.Err(); scanErr != nil {
			if isExpectedHLSStderrClose(scanErr) {
				return
			}
			appendTail(fmt.Sprintf("stderr scan error: %v", scanErr))
			_, _ = io.Copy(io.Discard, stderrPipe)
		}
	}()

	startErr := cmd.Start()
	if startErr != nil {
		stderrWg.Wait()
		return nil, startErr
	}

	go func() {
		exitErr := cmd.Wait()
		stderrWg.Wait()
		if onExit != nil {
			n := helpers.HLS_STDERR_TAIL_LINES
			if ringIdx < n {
				n = ringIdx
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
