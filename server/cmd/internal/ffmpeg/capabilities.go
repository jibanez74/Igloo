package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"igloo/cmd/internal/helpers"
)

const ffmpegProbeTimeout = 5 * time.Second

type Capabilities struct {
	Probed                         bool
	Encoders                       map[string]bool
	Filters                        map[string]bool
	HWAccels                       map[string]bool
	FilterOptions                  map[string]map[string]bool
	EncoderOptions                 map[string]map[string]bool
	CLIOptions                     map[string]bool
	H264NVENCRuntimeUsable         bool
	H264NVENCProbeError            string
	NvidiaCUDAScaleRuntimeUsable   bool
	NvidiaCUDAScaleProbeError      string
	NvidiaCUDATonemapRuntimeUsable bool
	NvidiaCUDATonemapProbeError    string
	H264QSVRuntimeUsable           bool
	H264QSVProbeError              string
	QSVScaleRuntimeUsable          bool
	QSVScaleProbeError             string
}

type HLSDeviceDecision struct {
	Configured string
	Effective  string
	Reason     string
}

func (c Capabilities) SupportsEncoder(name string) bool {
	return c.Encoders[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsFilter(name string) bool {
	return c.Filters[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsHWAccel(name string) bool {
	return c.HWAccels[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsCLIOption(name string) bool {
	return c.CLIOptions[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsFilterOption(filter, option string) bool {
	options := c.FilterOptions[strings.ToLower(strings.TrimSpace(filter))]
	if options == nil {
		return false
	}
	return options[strings.ToLower(strings.TrimSpace(option))]
}

func (c Capabilities) SupportsEncoderOption(encoder, option string) bool {
	options := c.EncoderOptions[strings.ToLower(strings.TrimSpace(encoder))]
	if options == nil {
		return false
	}
	return options[strings.ToLower(strings.TrimSpace(option))]
}

func (c Capabilities) SupportsNvidiaCUDAFilters(tonemap bool) bool {
	if !c.Probed {
		return false
	}
	if !c.SupportsEncoder("h264_nvenc") ||
		!c.H264NVENCRuntimeUsable ||
		!c.SupportsHWAccel("cuda") ||
		!c.SupportsFilter("hwupload") ||
		!c.SupportsFilter("scale_cuda") ||
		!c.SupportsFilterOption("scale_cuda", "format") ||
		!c.NvidiaCUDAScaleRuntimeUsable {
		return false
	}
	if !tonemap {
		return true
	}
	if !c.SupportsFilter("tonemap_cuda") || !c.SupportsNvidiaCUDATonemapOptions() {
		return false
	}
	return c.NvidiaCUDATonemapRuntimeUsable
}

func (c Capabilities) SupportsIntelQSVScale() bool {
	if !c.Probed {
		return false
	}
	return c.SupportsEncoder("h264_qsv") &&
		c.H264QSVRuntimeUsable &&
		c.QSVScaleRuntimeUsable &&
		c.SupportsHWAccel("qsv") &&
		c.SupportsFilter("scale_qsv") &&
		c.SupportsFilterOption("scale_qsv", "format")
}

func ResolveHLSDevice(configured string, caps Capabilities) HLSDeviceDecision {
	cfg := strings.ToLower(strings.TrimSpace(configured))
	if cfg == "" {
		cfg = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
	}

	decision := HLSDeviceDecision{
		Configured: cfg,
		Effective:  cfg,
	}

	switch cfg {
	case helpers.HARDWARE_ACCELERATION_DEVICE_CPU:
		return decision
	case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
		if !caps.Probed {
			return decision
		}
		switch {
		case !caps.SupportsEncoder("h264_nvenc"):
			decision.Effective = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
			decision.Reason = "ffmpeg does not list h264_nvenc"
		case !caps.H264NVENCRuntimeUsable:
			decision.Effective = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
			decision.Reason = "h264_nvenc runtime probe failed"
			if caps.H264NVENCProbeError != "" {
				decision.Reason += ": " + caps.H264NVENCProbeError
			}
		}
	case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
		if caps.Probed {
			switch {
			case !caps.SupportsEncoder("h264_qsv"):
				decision.Effective = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
				decision.Reason = "ffmpeg does not list h264_qsv"
			case !caps.H264QSVRuntimeUsable:
				decision.Effective = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
				decision.Reason = "h264_qsv runtime probe failed"
				if caps.H264QSVProbeError != "" {
					decision.Reason += ": " + caps.H264QSVProbeError
				}
			}
		}
	case helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
		if caps.Probed && !caps.SupportsEncoder("h264_videotoolbox") {
			decision.Effective = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
			decision.Reason = "ffmpeg does not list h264_videotoolbox"
		}
	default:
		decision.Effective = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
		decision.Reason = fmt.Sprintf("unknown hardware acceleration device %q", configured)
	}

	return decision
}

func probeCapabilities(bin string) Capabilities {
	caps := Capabilities{
		Probed:         true,
		Encoders:       map[string]bool{},
		Filters:        map[string]bool{},
		HWAccels:       map[string]bool{},
		FilterOptions:  map[string]map[string]bool{},
		EncoderOptions: map[string]map[string]bool{},
		CLIOptions:     map[string]bool{},
	}

	encoders, err := runFFmpegProbe(bin, "-encoders")
	if err == nil {
		caps.Encoders = parseFFmpegNamedRows(encoders)
	}

	filters, err := runFFmpegProbe(bin, "-filters")
	if err == nil {
		caps.Filters = parseFFmpegNamedRows(filters)
	}

	hwaccels, err := runFFmpegProbe(bin, "-hwaccels")
	if err == nil {
		caps.HWAccels = parseFFmpegHWAccels(hwaccels)
	}

	caps.recordCLIOptions(bin, []string{"readrate", "readrate_initial_burst"})

	caps.recordFilterOptions(bin, "scale_cuda", []string{"format"})
	caps.recordFilterOptions(bin, "tonemap_cuda", []string{"format", "p", "t", "m", "tonemap", "desat"})
	caps.recordFilterOptions(bin, "scale_qsv", []string{"format"})
	caps.recordEncoderOptions(bin, "h264_qsv", []string{"look_ahead", "forced_idr", "preset"})

	if caps.SupportsEncoder("h264_nvenc") {
		caps.H264NVENCRuntimeUsable, caps.H264NVENCProbeError = probeH264NVENC(bin)
		if caps.H264NVENCRuntimeUsable &&
			caps.SupportsHWAccel("cuda") &&
			caps.SupportsFilter("hwupload") &&
			caps.SupportsFilter("scale_cuda") &&
			caps.SupportsFilterOption("scale_cuda", "format") {
			caps.NvidiaCUDAScaleRuntimeUsable, caps.NvidiaCUDAScaleProbeError = probeNvidiaCUDAScale(bin)
		}
		if caps.NvidiaCUDAScaleRuntimeUsable &&
			caps.SupportsFilter("tonemap_cuda") &&
			caps.SupportsNvidiaCUDATonemapOptions() {
			caps.NvidiaCUDATonemapRuntimeUsable, caps.NvidiaCUDATonemapProbeError = probeNvidiaCUDATonemap(bin)
		}
	}
	if caps.SupportsEncoder("h264_qsv") {
		caps.H264QSVRuntimeUsable, caps.H264QSVProbeError = probeH264QSV(bin)
		if caps.H264QSVRuntimeUsable &&
			caps.SupportsHWAccel("qsv") &&
			caps.SupportsFilter("scale_qsv") &&
			caps.SupportsFilterOption("scale_qsv", "format") {
			caps.QSVScaleRuntimeUsable, caps.QSVScaleProbeError = probeQSVScale(bin)
		}
	}

	return caps
}

func (c Capabilities) SupportsNvidiaCUDATonemapOptions() bool {
	for _, option := range []string{"format", "p", "t", "m", "tonemap", "desat"} {
		if !c.SupportsFilterOption("tonemap_cuda", option) {
			return false
		}
	}
	return true
}

func (c *Capabilities) recordCLIOptions(bin string, options []string) {
	output, err := runFFmpegProbe(bin, "-hide_banner", "-h", "long")
	if err != nil {
		return
	}
	if c.CLIOptions == nil {
		c.CLIOptions = map[string]bool{}
	}
	for _, option := range options {
		option = strings.ToLower(strings.TrimSpace(option))
		c.CLIOptions[option] = ffmpegHelpHasOption(output, option)
	}
}

func (c *Capabilities) recordFilterOptions(bin string, filter string, options []string) {
	if !c.SupportsFilter(filter) {
		return
	}
	output, err := runFFmpegProbe(bin, "-h", "filter="+filter)
	if err != nil {
		return
	}
	key := strings.ToLower(filter)
	if c.FilterOptions[key] == nil {
		c.FilterOptions[key] = map[string]bool{}
	}
	for _, option := range options {
		option = strings.ToLower(strings.TrimSpace(option))
		c.FilterOptions[key][option] = ffmpegHelpHasOption(output, option)
	}
}

func (c *Capabilities) recordEncoderOptions(bin string, encoder string, options []string) {
	if !c.SupportsEncoder(encoder) {
		return
	}
	output, err := runFFmpegProbe(bin, "-h", "encoder="+encoder)
	if err != nil {
		return
	}
	key := strings.ToLower(encoder)
	if c.EncoderOptions[key] == nil {
		c.EncoderOptions[key] = map[string]bool{}
	}
	for _, option := range options {
		option = strings.ToLower(strings.TrimSpace(option))
		c.EncoderOptions[key][option] = ffmpegHelpHasOption(output, option)
	}
}

func ffmpegHelpHasOption(output string, option string) bool {
	option = strings.ToLower(strings.TrimSpace(option))
	if option == "" {
		return false
	}

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimLeft(fields[0], "-")
		if strings.EqualFold(name, option) {
			return true
		}
	}
	return false
}

func probeH264NVENC(bin string) (bool, string) {
	_, err := runFFmpegProbe(
		bin,
		"-v", "error",
		"-f", "lavfi",
		"-i", "testsrc2=s=128x72:d=0.1",
		"-frames:v", "1",
		"-c:v", "h264_nvenc",
		"-f", "null",
		"-",
	)
	if err != nil {
		return false, compactProbeError(err)
	}
	return true, ""
}

func probeNvidiaCUDAScale(bin string) (bool, string) {
	_, err := runFFmpegProbe(
		bin,
		"-v", "error",
		"-init_hw_device", "cuda="+hlsNvidiaCUDADeviceName,
		"-filter_hw_device", hlsNvidiaCUDADeviceName,
		"-f", "lavfi",
		"-i", "testsrc2=s=128x72:d=0.1",
		"-frames:v", "1",
		"-vf", "format=nv12,hwupload,scale_cuda=w=-2:h=72:format=yuv420p",
		"-c:v", "h264_nvenc",
		"-f", "null",
		"-",
	)
	if err != nil {
		return false, compactProbeError(err)
	}
	return true, ""
}

func probeNvidiaCUDATonemap(bin string) (bool, string) {
	_, err := runFFmpegProbe(
		bin,
		"-v", "error",
		"-init_hw_device", "cuda="+hlsNvidiaCUDADeviceName,
		"-filter_hw_device", hlsNvidiaCUDADeviceName,
		"-f", "lavfi",
		"-i", "testsrc2=s=128x72:d=0.1",
		"-frames:v", "1",
		"-vf", "format=p010le,hwupload,scale_cuda=w=-2:h=72:format=p010,tonemap_cuda=format=yuv420p:p=bt709:t=bt709:m=bt709:tonemap=hable:desat=0",
		"-c:v", "h264_nvenc",
		"-f", "null",
		"-",
	)
	if err != nil {
		return false, compactProbeError(err)
	}
	return true, ""
}

func probeH264QSV(bin string) (bool, string) {
	_, err := runFFmpegProbe(
		bin,
		"-v", "error",
		"-f", "lavfi",
		"-i", "testsrc2=s=128x72:d=0.1",
		"-frames:v", "1",
		"-vf", "format=nv12",
		"-c:v", "h264_qsv",
		"-f", "null",
		"-",
	)
	if err != nil {
		return false, compactProbeError(err)
	}
	return true, ""
}

func probeQSVScale(bin string) (bool, string) {
	_, err := runFFmpegProbe(
		bin,
		"-v", "error",
		"-init_hw_device", "qsv="+hlsIntelQSVDeviceName,
		"-filter_hw_device", hlsIntelQSVDeviceName,
		"-f", "lavfi",
		"-i", "testsrc2=s=128x72:d=0.1",
		"-frames:v", "1",
		"-vf", "format=nv12,hwupload=extra_hw_frames=64,scale_qsv=w=-2:h=72:format=nv12",
		"-c:v", "h264_qsv",
		"-f", "null",
		"-",
	)
	if err != nil {
		return false, compactProbeError(err)
	}
	return true, ""
}

func runFFmpegProbe(bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ffmpegProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), ctx.Err()
	}
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func parseFFmpegNamedRows(output string) map[string]bool {
	found := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		flags := fields[0]
		if len(flags) < 2 || strings.HasPrefix(flags, "-") {
			continue
		}
		name := strings.ToLower(fields[1])
		if name != "" && !strings.Contains(name, "=") {
			found[name] = true
		}
	}
	return found
}

func parseFFmpegHWAccels(output string) map[string]bool {
	found := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		name := strings.ToLower(strings.TrimSpace(line))
		if name == "" || strings.Contains(name, "hardware acceleration") {
			continue
		}
		found[name] = true
	}
	return found
}

func compactProbeError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 240 {
		msg = msg[:240]
	}
	return msg
}
