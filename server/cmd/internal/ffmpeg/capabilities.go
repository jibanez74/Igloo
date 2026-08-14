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

// ffmpegUnknownVersion stands in when the `-version` banner is missing or
// unparseable. It is a stable value on purpose: an unreadable banner must not
// churn the remux-safety fingerprints that embed it.
const ffmpegUnknownVersion = "unknown"

type Capabilities struct {
	Probed bool
	// Version is the token FFmpeg prints in its `-version` banner (for example
	// "7.0.2-Jellyfin"), or "unknown" when the banner could not be read. It
	// identifies the muxer that produced a remux-safety verdict, so a binary
	// swap or upgrade invalidates verdicts it did not produce.
	Version                        string
	Encoders                       map[string]bool
	Filters                        map[string]bool
	HWAccels                       map[string]bool
	FilterOptions                  map[string]map[string]bool
	EncoderOptions                 map[string]map[string]bool
	MuxerFlags                     map[string]map[string]bool
	CLIOptions                     map[string]bool
	H264NVENCRuntimeUsable         bool
	H264NVENCProbeError            string
	NvidiaCUDAScaleRuntimeUsable   bool
	NvidiaCUDATonemapRuntimeUsable bool
	H264QSVRuntimeUsable           bool
	H264QSVProbeError              string
	QSVScaleRuntimeUsable          bool
}

type HLSDeviceDecision struct {
	Configured string
	Effective  string
	Reason     string
}

func (c Capabilities) SupportsEncoder(name string) bool {
	return c.Encoders[strings.ToLower(strings.TrimSpace(name))]
}

func cloneCapabilities(source Capabilities) Capabilities {
	cloned := source
	cloned.Encoders = cloneBoolMap(source.Encoders)
	cloned.Filters = cloneBoolMap(source.Filters)
	cloned.HWAccels = cloneBoolMap(source.HWAccels)
	cloned.CLIOptions = cloneBoolMap(source.CLIOptions)
	cloned.FilterOptions = cloneNestedBoolMap(source.FilterOptions)
	cloned.EncoderOptions = cloneNestedBoolMap(source.EncoderOptions)
	cloned.MuxerFlags = cloneNestedBoolMap(source.MuxerFlags)
	return cloned
}

func cloneBoolMap(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	cloned := make(map[string]bool, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneNestedBoolMap(source map[string]map[string]bool) map[string]map[string]bool {
	if source == nil {
		return nil
	}
	cloned := make(map[string]map[string]bool, len(source))
	for key, value := range source {
		cloned[key] = cloneBoolMap(value)
	}
	return cloned
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

func (c Capabilities) SupportsMuxerFlag(muxer, flag string) bool {
	flags := c.MuxerFlags[strings.ToLower(strings.TrimSpace(muxer))]
	if flags == nil {
		return false
	}
	return flags[strings.ToLower(strings.TrimSpace(flag))]
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

// probeCapabilities inspects one FFmpeg binary. versionOutput is the `-version`
// banner the caller already ran to prove the binary executes, reused here so
// startup does not invoke it twice.
func probeCapabilities(bin string, versionOutput string) Capabilities {
	caps := Capabilities{
		Probed:         true,
		Version:        parseFFmpegVersion(versionOutput),
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
	// NVENC spells the option with a hyphen while QSV uses an underscore. Both
	// default to false, which turns a forced keyframe into a non-IDR I-frame.
	caps.recordEncoderOptions(bin, "h264_nvenc", []string{"forced-idr"})
	// temp_file makes the hls muxer write each segment to a .tmp name and
	// rename it on close, which is what lets segment readiness be judged by
	// the final name's existence alone.
	caps.recordMuxerFlags(bin, "hls", []string{"temp_file"})

	if caps.SupportsEncoder("h264_nvenc") {
		caps.H264NVENCRuntimeUsable, caps.H264NVENCProbeError = probeH264NVENC(bin)
		if caps.H264NVENCRuntimeUsable &&
			caps.SupportsHWAccel("cuda") &&
			caps.SupportsFilter("hwupload") &&
			caps.SupportsFilter("scale_cuda") &&
			caps.SupportsFilterOption("scale_cuda", "format") {
			caps.NvidiaCUDAScaleRuntimeUsable = probeNvidiaCUDAScale(bin)
		}
		if caps.NvidiaCUDAScaleRuntimeUsable &&
			caps.SupportsFilter("tonemap_cuda") &&
			caps.SupportsNvidiaCUDATonemapOptions() {
			caps.NvidiaCUDATonemapRuntimeUsable = probeNvidiaCUDATonemap(bin)
		}
	}
	if caps.SupportsEncoder("h264_qsv") {
		caps.H264QSVRuntimeUsable, caps.H264QSVProbeError = probeH264QSV(bin)
		if caps.H264QSVRuntimeUsable &&
			caps.SupportsHWAccel("qsv") &&
			caps.SupportsFilter("scale_qsv") &&
			caps.SupportsFilterOption("scale_qsv", "format") {
			caps.QSVScaleRuntimeUsable = probeQSVScale(bin)
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
	if c.FilterOptions == nil {
		c.FilterOptions = map[string]map[string]bool{}
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
	if c.EncoderOptions == nil {
		c.EncoderOptions = map[string]map[string]bool{}
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

func (c *Capabilities) recordMuxerFlags(bin string, muxer string, flags []string) {
	output, err := runFFmpegProbe(bin, "-hide_banner", "-h", "muxer="+muxer)
	if err != nil {
		return
	}
	if c.MuxerFlags == nil {
		c.MuxerFlags = map[string]map[string]bool{}
	}
	key := strings.ToLower(muxer)
	if c.MuxerFlags[key] == nil {
		c.MuxerFlags[key] = map[string]bool{}
	}
	for _, flag := range flags {
		flag = strings.ToLower(strings.TrimSpace(flag))
		c.MuxerFlags[key][flag] = ffmpegHelpHasOption(output, flag)
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

func probeNvidiaCUDAScale(bin string) bool {
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
		return false
	}
	return true
}

func probeNvidiaCUDATonemap(bin string) bool {
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
		return false
	}
	return true
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

func probeQSVScale(bin string) bool {
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
		return false
	}
	return true
}

func runFFmpegProbe(bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ffmpegProbeTimeout)
	defer cancel()
	return runFFmpegProbeContext(ctx, bin, args...)
}

func runFFmpegProbeContext(ctx context.Context, bin string, args ...string) (string, error) {
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

// parseFFmpegVersion pulls the version token out of the `-version` banner,
// whose first line reads "ffmpeg version <token> Copyright ...". Anything that
// does not match that shape yields ffmpegUnknownVersion rather than a partial
// string, so the value stays stable across probes of the same binary.
func parseFFmpegVersion(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if !strings.EqualFold(fields[0], "ffmpeg") || !strings.EqualFold(fields[1], "version") {
			continue
		}
		return fields[2]
	}
	return ffmpegUnknownVersion
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
