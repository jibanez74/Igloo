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
	Probed                 bool
	Version                string
	ProbeError             string
	Encoders               map[string]bool
	Decoders               map[string]bool
	Filters                map[string]bool
	HWAccels               map[string]bool
	FilterOptions          map[string]map[string]bool
	H264NVENCRuntimeUsable bool
	H264NVENCProbeError    string
}

type HLSDeviceDecision struct {
	Configured string
	Effective  string
	Reason     string
}

func (c Capabilities) SupportsEncoder(name string) bool {
	return c.Encoders[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsDecoder(name string) bool {
	return c.Decoders[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsFilter(name string) bool {
	return c.Filters[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsHWAccel(name string) bool {
	return c.HWAccels[strings.ToLower(strings.TrimSpace(name))]
}

func (c Capabilities) SupportsFilterOption(filter, option string) bool {
	options := c.FilterOptions[strings.ToLower(strings.TrimSpace(filter))]
	if options == nil {
		return false
	}
	return options[strings.ToLower(strings.TrimSpace(option))]
}

func (c Capabilities) SupportsNvidiaCUDAFilters(tonemap bool) bool {
	if !c.Probed {
		return false
	}
	if !c.SupportsHWAccel("cuda") || !c.SupportsFilter("scale_cuda") || !c.SupportsFilterOption("scale_cuda", "format") {
		return false
	}
	if !tonemap {
		return true
	}
	if !c.SupportsFilter("tonemap_cuda") {
		return false
	}
	for _, option := range []string{"format", "p", "t", "m", "tonemap", "desat"} {
		if !c.SupportsFilterOption("tonemap_cuda", option) {
			return false
		}
	}
	return true
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
		if caps.Probed && !caps.SupportsEncoder("h264_qsv") {
			decision.Effective = helpers.HARDWARE_ACCELERATION_DEVICE_CPU
			decision.Reason = "ffmpeg does not list h264_qsv"
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
		Probed:        true,
		Encoders:      map[string]bool{},
		Decoders:      map[string]bool{},
		Filters:       map[string]bool{},
		HWAccels:      map[string]bool{},
		FilterOptions: map[string]map[string]bool{},
	}

	version, err := runFFmpegProbe(bin, "-version")
	if err != nil {
		caps.ProbeError = appendProbeError(caps.ProbeError, "version", err)
	} else {
		caps.Version = firstNonEmptyLine(version)
	}

	encoders, err := runFFmpegProbe(bin, "-encoders")
	if err != nil {
		caps.ProbeError = appendProbeError(caps.ProbeError, "encoders", err)
	} else {
		caps.Encoders = parseFFmpegNamedRows(encoders)
	}

	decoders, err := runFFmpegProbe(bin, "-decoders")
	if err != nil {
		caps.ProbeError = appendProbeError(caps.ProbeError, "decoders", err)
	} else {
		caps.Decoders = parseFFmpegNamedRows(decoders)
	}

	filters, err := runFFmpegProbe(bin, "-filters")
	if err != nil {
		caps.ProbeError = appendProbeError(caps.ProbeError, "filters", err)
	} else {
		caps.Filters = parseFFmpegNamedRows(filters)
	}

	hwaccels, err := runFFmpegProbe(bin, "-hwaccels")
	if err != nil {
		caps.ProbeError = appendProbeError(caps.ProbeError, "hwaccels", err)
	} else {
		caps.HWAccels = parseFFmpegHWAccels(hwaccels)
	}

	caps.recordFilterOptions(bin, "scale_cuda", []string{"format"})
	caps.recordFilterOptions(bin, "tonemap_cuda", []string{"format", "p", "t", "m", "tonemap", "desat"})

	if caps.SupportsEncoder("h264_nvenc") {
		caps.H264NVENCRuntimeUsable, caps.H264NVENCProbeError = probeH264NVENC(bin)
	}

	return caps
}

func (c *Capabilities) recordFilterOptions(bin string, filter string, options []string) {
	if !c.SupportsFilter(filter) {
		return
	}
	output, err := runFFmpegProbe(bin, "-h", "filter="+filter)
	if err != nil {
		c.ProbeError = appendProbeError(c.ProbeError, "filter="+filter, err)
		return
	}
	key := strings.ToLower(filter)
	if c.FilterOptions[key] == nil {
		c.FilterOptions[key] = map[string]bool{}
	}
	for _, option := range options {
		option = strings.ToLower(strings.TrimSpace(option))
		c.FilterOptions[key][option] = ffmpegFilterHelpHasOption(output, option)
	}
}

func ffmpegFilterHelpHasOption(output string, option string) bool {
	option = strings.ToLower(strings.TrimSpace(option))
	if option == "" {
		return false
	}

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		if strings.EqualFold(fields[0], option) {
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
		"-i", "color=c=black:s=16x16:d=0.1",
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

func appendProbeError(existing, name string, err error) string {
	message := name + ": " + compactProbeError(err)
	if existing == "" {
		return message
	}
	return existing + "; " + message
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

func firstNonEmptyLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
