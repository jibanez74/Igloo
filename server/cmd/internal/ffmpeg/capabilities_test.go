package ffmpeg

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/fakebin"
	"igloo/cmd/internal/helpers"
)

func TestFFmpegHelpHasOption(t *testing.T) {
	filterHelp := `Filter tonemap_cuda
  Inputs:
     #0: default (video)
  tonemap_cuda AVOptions:
   format            <pix_fmt>    ..FV....... Output pixel format
   tonemap           <int>        ..FV....... Tonemap algorithm
`
	encoderHelp := `Encoder h264_qsv
  h264_qsv AVOptions:
   -preset           <int>        E..V....... Encoding preset
   -look_ahead       <boolean>    E..V....... Use lookahead
`
	cliHelp := "\n   \n-readrate value\nreadrate_initial_burst value\n"

	tests := []struct {
		name   string
		output string
		option string
		want   bool
	}{
		{name: "filter option", output: filterHelp, option: "format", want: true},
		// "p" appears inside several help words; only a whole option name counts.
		{name: "single letter does not match arbitrary words", output: filterHelp, option: "p"},
		{name: "dashed encoder option", output: encoderHelp, option: "look_ahead", want: true},
		{name: "partial option name", output: encoderHelp, option: "look"},
		{name: "blank option", output: cliHelp, option: ""},
		{name: "dashed option is normalized", output: cliHelp, option: " READRATE ", want: true},
		{name: "partial CLI option", output: cliHelp, option: "rate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ffmpegHelpHasOption(tt.output, tt.option)
			if got != tt.want {
				t.Fatalf("ffmpegHelpHasOption(%q) = %v, want %v", tt.option, got, tt.want)
			}
		})
	}
}

// The parsed version keys persisted remux-safety verdicts, so an unreadable
// banner has to collapse to one stable value rather than a partial string that
// would churn fingerprints and re-run preflight on every play.
func TestParseFFmpegVersion(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "jellyfin build",
			output: "ffmpeg version 7.0.2-Jellyfin Copyright (c) 2000-2024 the FFmpeg developers\nbuilt with gcc 13\n",
			want:   "7.0.2-Jellyfin",
		},
		{
			name:   "distribution build",
			output: "ffmpeg version 6.1.1-3ubuntu5 Copyright (c) 2000-2023 the FFmpeg developers\n",
			want:   "6.1.1-3ubuntu5",
		},
		{
			name:   "banner after a leading notice",
			output: "some wrapper notice\nffmpeg version n7.1 Copyright (c) 2000-2024\n",
			want:   "n7.1",
		},
		{name: "empty output", output: "", want: ffmpegUnknownVersion},
		{name: "unexpected shape", output: "ffmpeg 7.0.2\n", want: ffmpegUnknownVersion},
		{name: "truncated banner", output: "ffmpeg version\n", want: ffmpegUnknownVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFFmpegVersion(tt.output)
			if got != tt.want {
				t.Fatalf("parseFFmpegVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSupportsNvidiaCUDAFilters(t *testing.T) {
	base := func() Capabilities {
		return Capabilities{
			Probed:   true,
			Encoders: map[string]bool{"h264_nvenc": true},
			HWAccels: map[string]bool{"cuda": true},
			Filters:  map[string]bool{"hwupload": true, "scale_cuda": true, "tonemap_cuda": true},
			FilterOptions: map[string]map[string]bool{
				"scale_cuda": {"format": true},
				"tonemap_cuda": {
					"format":  true,
					"p":       true,
					"t":       true,
					"m":       true,
					"tonemap": true,
					"desat":   true,
				},
			},
			H264NVENCRuntimeUsable:         true,
			NvidiaCUDAScaleRuntimeUsable:   true,
			NvidiaCUDATonemapRuntimeUsable: true,
		}
	}

	tests := []struct {
		name    string
		tonemap bool
		mutate  func(*Capabilities)
		want    bool
	}{
		{name: "scale supported", want: true},
		{name: "tone-map supported", tonemap: true, want: true},
		{
			name:   "not probed",
			mutate: func(c *Capabilities) { c.Probed = false },
		},
		{
			name:   "missing h264_nvenc encoder",
			mutate: func(c *Capabilities) { delete(c.Encoders, "h264_nvenc") },
		},
		{
			name:   "nvenc runtime failed",
			mutate: func(c *Capabilities) { c.H264NVENCRuntimeUsable = false },
		},
		{
			name:   "missing cuda hwaccel",
			mutate: func(c *Capabilities) { c.HWAccels = map[string]bool{} },
		},
		{
			name:   "missing hwupload filter",
			mutate: func(c *Capabilities) { delete(c.Filters, "hwupload") },
		},
		{
			name:   "missing scale_cuda format option",
			mutate: func(c *Capabilities) { c.FilterOptions["scale_cuda"] = map[string]bool{} },
		},
		{
			name:   "scale runtime probe failed",
			mutate: func(c *Capabilities) { c.NvidiaCUDAScaleRuntimeUsable = false },
		},
		{
			name:    "tone-map needs the tonemap_cuda filter",
			tonemap: true,
			mutate:  func(c *Capabilities) { delete(c.Filters, "tonemap_cuda") },
		},
		{
			// tonemap_cuda must expose every option the filter string uses.
			name:    "tone-map needs the full option set",
			tonemap: true,
			mutate:  func(c *Capabilities) { c.FilterOptions["tonemap_cuda"] = map[string]bool{"format": true} },
		},
		{
			name:    "tone-map runtime probe failed",
			tonemap: true,
			mutate:  func(c *Capabilities) { c.NvidiaCUDATonemapRuntimeUsable = false },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := base()
			if tt.mutate != nil {
				tt.mutate(&caps)
			}

			got := caps.SupportsNvidiaCUDAFilters(tt.tonemap)
			if got != tt.want {
				t.Fatalf("SupportsNvidiaCUDAFilters(%v) = %v, want %v", tt.tonemap, got, tt.want)
			}
		})
	}
}

func TestSupportsIntelQSVScaleRequiresEncodeAndScaleSupport(t *testing.T) {
	base := func() Capabilities {
		return Capabilities{
			Probed:                true,
			Encoders:              map[string]bool{"h264_qsv": true},
			HWAccels:              map[string]bool{"qsv": true},
			Filters:               map[string]bool{"scale_qsv": true},
			FilterOptions:         map[string]map[string]bool{"scale_qsv": {"format": true}},
			H264QSVRuntimeUsable:  true,
			QSVScaleRuntimeUsable: true,
		}
	}

	tests := []struct {
		name   string
		mutate func(*Capabilities)
		want   bool
	}{
		{
			name: "supported",
			want: true,
		},
		{
			name: "not probed",
			mutate: func(c *Capabilities) {
				c.Probed = false
			},
		},
		{
			name: "missing h264_qsv encoder",
			mutate: func(c *Capabilities) {
				delete(c.Encoders, "h264_qsv")
			},
		},
		{
			name: "h264_qsv runtime failed",
			mutate: func(c *Capabilities) {
				c.H264QSVRuntimeUsable = false
			},
		},
		{
			name: "missing qsv hwaccel",
			mutate: func(c *Capabilities) {
				c.HWAccels = map[string]bool{}
			},
		},
		{
			name: "missing scale_qsv filter",
			mutate: func(c *Capabilities) {
				c.Filters = map[string]bool{}
			},
		},
		{
			name: "missing scale_qsv format option",
			mutate: func(c *Capabilities) {
				c.FilterOptions["scale_qsv"] = map[string]bool{}
			},
		},
		{
			name: "scale runtime failed",
			mutate: func(c *Capabilities) {
				c.QSVScaleRuntimeUsable = false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := base()
			if tt.mutate != nil {
				tt.mutate(&caps)
			}

			got := caps.SupportsIntelQSVScale()
			if got != tt.want {
				t.Fatalf("SupportsIntelQSVScale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportsAppleVTTonemapRequiresTheRuntimeProbe(t *testing.T) {
	base := func() Capabilities {
		return Capabilities{
			Probed:                      true,
			Encoders:                    map[string]bool{"h264_videotoolbox": true},
			Filters:                     map[string]bool{"hwupload": true, "scale_vt": true},
			AppleVTTonemapRuntimeUsable: true,
		}
	}

	tests := []struct {
		name   string
		mutate func(*Capabilities)
		want   bool
	}{
		{name: "supported", want: true},
		{name: "not probed", mutate: func(c *Capabilities) { c.Probed = false }},
		{name: "missing h264_videotoolbox encoder", mutate: func(c *Capabilities) { c.Encoders = map[string]bool{} }},
		{name: "missing hwupload filter", mutate: func(c *Capabilities) { delete(c.Filters, "hwupload") }},
		{name: "missing scale_vt filter", mutate: func(c *Capabilities) { delete(c.Filters, "scale_vt") }},
		{name: "runtime probe failed", mutate: func(c *Capabilities) { c.AppleVTTonemapRuntimeUsable = false }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := base()
			if tt.mutate != nil {
				tt.mutate(&caps)
			}

			got := caps.SupportsAppleVTTonemap()
			if got != tt.want {
				t.Fatalf("SupportsAppleVTTonemap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveHLSDeviceScenarios(t *testing.T) {
	tests := []struct {
		name           string
		configured     string
		caps           Capabilities
		wantConfigured string
		wantEffective  string
		wantReason     string
	}{
		{
			name:           "blank defaults to CPU",
			configured:     "  ",
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		},
		{
			name:           "CPU is normalized",
			configured:     " CPU ",
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
		},
		{
			name:           "unprobed NVIDIA remains configured",
			configured:     " NVIDIA ",
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		},
		{
			name:           "missing NVENC",
			configured:     helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps:           Capabilities{Probed: true},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantReason:     "does not list h264_nvenc",
		},
		{
			name:       "NVENC runtime probe failed",
			configured: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps: Capabilities{
				Probed:                 true,
				Encoders:               map[string]bool{"h264_nvenc": true},
				H264NVENCRuntimeUsable: false,
				H264NVENCProbeError:    "no capable devices",
			},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantReason:     "runtime probe failed: no capable devices",
		},
		{
			name:       "usable NVENC",
			configured: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			caps: Capabilities{
				Probed:                 true,
				Encoders:               map[string]bool{"h264_nvenc": true},
				H264NVENCRuntimeUsable: true,
			},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA,
		},
		{
			name:           "missing QSV",
			configured:     helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps:           Capabilities{Probed: true},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantReason:     "does not list h264_qsv",
		},
		{
			name:       "QSV runtime probe failed",
			configured: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps: Capabilities{
				Probed:               true,
				Encoders:             map[string]bool{"h264_qsv": true},
				H264QSVRuntimeUsable: false,
				H264QSVProbeError:    "no qsv device",
			},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantReason:     "runtime probe failed: no qsv device",
		},
		{
			name:       "usable QSV",
			configured: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			caps: Capabilities{
				Probed:               true,
				Encoders:             map[string]bool{"h264_qsv": true},
				H264QSVRuntimeUsable: true,
			},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_INTEL,
		},
		{
			name:           "missing VideoToolbox",
			configured:     helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			caps:           Capabilities{Probed: true},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantReason:     "does not list h264_videotoolbox",
		},
		{
			name:       "usable VideoToolbox",
			configured: helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			caps: Capabilities{
				Probed:   true,
				Encoders: map[string]bool{"h264_videotoolbox": true},
			},
			wantConfigured: helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_APPLE,
		},
		{
			name:           "unknown falls back",
			configured:     " Mystery ",
			caps:           Capabilities{Probed: true},
			wantConfigured: "mystery",
			wantEffective:  helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			wantReason:     "unknown hardware acceleration device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := ResolveHLSDevice(tt.configured, tt.caps)
			if decision.Configured != tt.wantConfigured {
				t.Fatalf("Configured = %q, want %q", decision.Configured, tt.wantConfigured)
			}
			if decision.Effective != tt.wantEffective {
				t.Fatalf("Effective = %q, want %q", decision.Effective, tt.wantEffective)
			}
			if !strings.Contains(decision.Reason, tt.wantReason) {
				t.Fatalf("Reason = %q, want substring %q", decision.Reason, tt.wantReason)
			}
		})
	}
}

func TestCapabilityLookupsNormalizeNamesAndHandleNilMaps(t *testing.T) {
	caps := Capabilities{
		Encoders:       map[string]bool{"h264_nvenc": true},
		Filters:        map[string]bool{"scale_cuda": true},
		HWAccels:       map[string]bool{"cuda": true},
		CLIOptions:     map[string]bool{"readrate": true},
		FilterOptions:  map[string]map[string]bool{"scale_cuda": {"format": true}},
		EncoderOptions: map[string]map[string]bool{"h264_qsv": {"look_ahead": true}},
		MuxerFlags:     map[string]map[string]bool{"hls": {"temp_file": true}},
	}

	if !caps.SupportsEncoder(" H264_NVENC ") || !caps.SupportsFilter(" SCALE_CUDA ") {
		t.Fatal("encoder/filter names were not normalized")
	}
	if !caps.SupportsHWAccel(" CUDA ") || !caps.SupportsCLIOption(" READRATE ") {
		t.Fatal("hardware/CLI option names were not normalized")
	}
	if !caps.SupportsFilterOption(" SCALE_CUDA ", " FORMAT ") {
		t.Fatal("filter option names were not normalized")
	}
	if !caps.SupportsEncoderOption(" H264_QSV ", " LOOK_AHEAD ") {
		t.Fatal("encoder option names were not normalized")
	}
	if !caps.SupportsMuxerFlag(" HLS ", " TEMP_FILE ") {
		t.Fatal("muxer flag names were not normalized")
	}

	empty := Capabilities{}
	if empty.SupportsEncoder("x") || empty.SupportsFilter("x") || empty.SupportsHWAccel("x") {
		t.Fatal("nil capability maps reported support")
	}
	if empty.SupportsCLIOption("x") || empty.SupportsFilterOption("x", "y") {
		t.Fatal("nil option maps reported support")
	}
	if empty.SupportsEncoderOption("x", "y") || empty.SupportsMuxerFlag("x", "y") {
		t.Fatal("nil encoder option/muxer flag maps reported support")
	}
}

func TestCapabilityParsersHandleHeadersMalformedRowsAndCase(t *testing.T) {
	named := parseFFmpegNamedRows(`
Encoders:
 V..... H264_NVENC description
 V..... h264_qsv Intel encoder
 ... SCALE_CUDA V->V
 -bad rejected
 X
 .. name=value malformed
`)
	for _, want := range []string{"h264_nvenc", "h264_qsv", "scale_cuda"} {
		if !named[want] {
			t.Fatalf("missing %q in parsed named rows: %#v", want, named)
		}
	}
	if named["rejected"] || named["name=value"] {
		t.Fatalf("malformed rows were accepted: %#v", named)
	}

	hwaccels := parseFFmpegHWAccels("Hardware acceleration methods:\n CUDA \nQSV\n\n")
	if !hwaccels["cuda"] || !hwaccels["qsv"] || len(hwaccels) != 2 {
		t.Fatalf("parsed hardware accelerators = %#v", hwaccels)
	}
}

// fakeFFmpegVersionBanner stands in for the `-version` output that
// initializeCandidate captures before handing it to probeCapabilities.
const fakeFFmpegVersionBanner = "ffmpeg version 7.0.2-Jellyfin Copyright (c) 2000-2024 the FFmpeg developers\n"

func fullCapabilityProbeFake(t *testing.T, runtimeExit int, logPath string) string {
	t.Helper()
	runtimeResult := "exit 0"
	if runtimeExit != 0 {
		runtimeResult = "printf '%0300d' 0 >&2\nexit " + strconv.Itoa(runtimeExit)
	}
	body := fakebin.CallLogLine(logPath) + `
if [ "$1" = "-encoders" ]; then
  printf '%s\n' ' V..... h264_nvenc NVIDIA' ' V..... h264_qsv Intel' ' V..... libx264 CPU'
  exit 0
fi
if [ "$1" = "-filters" ]; then
  printf '%s\n' ' ... hwupload V->V' ' ... scale_cuda V->V' ' ... tonemap_cuda V->V' ' ... scale_qsv V->V'
  exit 0
fi
if [ "$1" = "-hwaccels" ]; then
  printf '%s\n' 'Hardware acceleration methods:' 'cuda' 'qsv'
  exit 0
fi
if [ "$1" = "-hide_banner" ] && [ "$3" = "muxer=hls" ]; then
  printf '%s\n' '  -hls_flags <flags> E... set flags' '     temp_file  E... write segment and playlist to temporary file and rename when complete'
  exit 0
fi
if [ "$1" = "-hide_banner" ]; then
  printf '%s\n' '-readrate value' '-readrate_initial_burst value'
  exit 0
fi
if [ "$1" = "-h" ] && [ "$2" = "filter=scale_cuda" ]; then
  printf '%s\n' 'format value'
  exit 0
fi
if [ "$1" = "-h" ] && [ "$2" = "filter=tonemap_cuda" ]; then
  printf '%s\n' 'format value' 'p value' 't value' 'm value' 'tonemap value' 'desat value'
  exit 0
fi
if [ "$1" = "-h" ] && [ "$2" = "filter=scale_qsv" ]; then
  printf '%s\n' 'format value'
  exit 0
fi
if [ "$1" = "-h" ] && [ "$2" = "encoder=h264_qsv" ]; then
  printf '%s\n' '-preset value' '-look_ahead value' '-forced_idr value'
  exit 0
fi
if [ "$1" = "-h" ] && [ "$2" = "encoder=h264_nvenc" ]; then
  printf '%s\n' '-rc value' '-preset value' '-forced-idr value'
  exit 0
fi
` + runtimeResult + "\n"
	return fakebin.Write(t, "probe ffmpeg", body)
}

func TestProbeCapabilitiesSuccessfulStaticAndRuntimeProbes(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "probes.log")
	script := fullCapabilityProbeFake(t, 0, logPath)

	caps := probeCapabilities(script, fakeFFmpegVersionBanner)
	if !caps.Probed || !caps.SupportsEncoder("h264_nvenc") || !caps.SupportsEncoder("h264_qsv") {
		t.Fatalf("encoder capabilities were not orchestrated: %#v", caps)
	}
	if !caps.SupportsFilter("scale_cuda") || !caps.SupportsHWAccel("qsv") {
		t.Fatalf("filter/hwaccel capabilities were not orchestrated: %#v", caps)
	}
	if !caps.SupportsCLIOption("readrate") || !caps.SupportsCLIOption("readrate_initial_burst") {
		t.Fatalf("CLI capabilities were not orchestrated: %#v", caps.CLIOptions)
	}
	if !caps.SupportsFilterOption("tonemap_cuda", "desat") {
		t.Fatalf("filter options were not orchestrated: %#v", caps.FilterOptions)
	}
	// NVENC spells the option with a hyphen, QSV with an underscore; both must
	// be recorded or the transcode silently loses its IDR guarantee.
	if !caps.SupportsEncoderOption("h264_qsv", "forced_idr") ||
		!caps.SupportsEncoderOption("h264_nvenc", "forced-idr") {
		t.Fatalf("encoder options were not orchestrated: %#v", caps.EncoderOptions)
	}
	if !caps.SupportsMuxerFlag("hls", "temp_file") {
		t.Fatalf("muxer flags were not orchestrated: %#v", caps.MuxerFlags)
	}
	if caps.Version != "7.0.2-Jellyfin" {
		t.Fatalf("Version = %q, want the token from the -version banner", caps.Version)
	}
	if !caps.H264NVENCRuntimeUsable || !caps.NvidiaCUDAScaleRuntimeUsable {
		t.Fatalf("NVENC/CUDA runtime probes did not succeed: %#v", caps)
	}
	if !caps.NvidiaCUDATonemapRuntimeUsable || !caps.H264QSVRuntimeUsable || !caps.QSVScaleRuntimeUsable {
		t.Fatalf("tone-map/QSV runtime probes did not succeed: %#v", caps)
	}

	// NVENC refuses an H.264 frame below a minimum that depends on the GPU
	// and driver; 145x49 is the one the QA server's GTX 1660 SUPER enforces.
	// A probe that hands it a smaller frame fails on hardware that works, and
	// the only symptom is that every transcode quietly runs on libx264. The
	// frame each probe encodes is its lavfi source, or the scale_cuda output
	// when the chain rescales it.
	const nvencMinWidth, nvencMinHeight = 145, 49
	sourceSize := regexp.MustCompile(`testsrc2=s=(\d+)x(\d+)`)
	scaledHeight := regexp.MustCompile(`scale_cuda=w=-2:h=(\d+)`)
	nvencProbes := 0
	for _, line := range fakebin.ReadLog(t, logPath) {
		isNVENCProbe := strings.Contains(line, "-c:v h264_nvenc") && strings.Contains(line, "testsrc2")
		if !isNVENCProbe {
			continue
		}
		nvencProbes++

		size := sourceSize.FindStringSubmatch(line)
		if size == nil {
			t.Fatalf("NVENC probe has no sized source: %s", line)
		}
		width, _ := strconv.Atoi(size[1])
		height, _ := strconv.Atoi(size[2])
		scaled := scaledHeight.FindStringSubmatch(line)
		if scaled != nil {
			scaledTo, _ := strconv.Atoi(scaled[1])
			width = width * scaledTo / height
			height = scaledTo
		}

		belowMinimum := width < nvencMinWidth || height < nvencMinHeight
		if belowMinimum {
			t.Errorf("NVENC probe encodes %dx%d, below the %dx%d minimum: %s",
				width, height, nvencMinWidth, nvencMinHeight, line)
		}
	}
	if nvencProbes != 3 {
		t.Fatalf("found %d NVENC runtime probes, want encode, CUDA scale, and CUDA tone-map", nvencProbes)
	}
}

// tonemap_cuda only accepts a PQ or HLG frame; an untagged lavfi frame made
// the probe fail on every GPU, whatever the hardware could do.
func TestNvidiaToneMapProbeTagsItsFrameAsHDR(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "tonemap.log")
	script := fakebin.Write(t, "probe ffmpeg", fakebin.ArgsLogLine(logPath)+"exit 0\n")

	if !probeNvidiaCUDATonemap(script) {
		t.Fatal("tone-map probe failed against a fake that accepts it")
	}
	args := fakebin.ReadLog(t, logPath)
	filterIndex := slices.Index(args, "-vf")
	if filterIndex < 0 || filterIndex+1 >= len(args) {
		t.Fatalf("tone-map probe has no filter chain: %q", args)
	}
	chain := args[filterIndex+1]
	tagIndex := strings.Index(chain, "color_trc=smpte2084")
	tonemapIndex := strings.Index(chain, "tonemap_cuda=")
	taggedFirst := tagIndex >= 0 && tagIndex < tonemapIndex
	if !taggedFirst {
		t.Fatalf("tone-map probe does not tag its frame as PQ before tonemap_cuda: %s", chain)
	}
}

func TestProbeCapabilitiesMissingPrerequisitesSuppressesFilterRuntimeProbes(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "probes.log")
	body := fakebin.CallLogLine(logPath) + `
if [ "$1" = "-encoders" ]; then
  printf '%s\n' ' V..... h264_nvenc NVIDIA' ' V..... h264_qsv Intel'
fi
exit 0
`
	script := fakebin.Write(t, "probe ffmpeg", body)

	caps := probeCapabilities(script, fakeFFmpegVersionBanner)
	if !caps.H264NVENCRuntimeUsable || !caps.H264QSVRuntimeUsable {
		t.Fatalf("encoder runtime probes should succeed in the fake: %#v", caps)
	}
	if caps.NvidiaCUDAScaleRuntimeUsable || caps.NvidiaCUDATonemapRuntimeUsable || caps.QSVScaleRuntimeUsable {
		t.Fatalf("filter runtime probes ran without prerequisites: %#v", caps)
	}

	invocations := fakebin.ReadLog(t, logPath)
	usedHWDevice := slices.ContainsFunc(invocations, func(line string) bool {
		return strings.Contains(line, "-init_hw_device")
	})
	if usedHWDevice {
		t.Fatalf("hardware filter runtime probe was not suppressed:\n%s", strings.Join(invocations, "\n"))
	}
}

func TestProbeCapabilitiesRecordsBoundedEncoderDiagnostics(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "probes.log")
	script := fullCapabilityProbeFake(t, 7, logPath)

	caps := probeCapabilities(script, fakeFFmpegVersionBanner)
	if caps.H264NVENCRuntimeUsable || caps.H264QSVRuntimeUsable {
		t.Fatalf("failing encoder runtime probes reported usable: %#v", caps)
	}
	if caps.H264NVENCProbeError == "" || caps.H264QSVProbeError == "" {
		t.Fatalf("encoder diagnostics were not retained: %#v", caps)
	}
	if len(caps.H264NVENCProbeError) > 240 || len(caps.H264QSVProbeError) > 240 {
		t.Fatalf("encoder diagnostics were not bounded: nvenc=%d qsv=%d", len(caps.H264NVENCProbeError), len(caps.H264QSVProbeError))
	}
}

func TestCompactProbeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "success is empty"},
		{name: "surrounding whitespace is trimmed", err: errors.New("  no device \n"), want: "no device"},
		{name: "long diagnostics are cut at 240 bytes", err: errors.New(strings.Repeat("x", 300)), want: strings.Repeat("x", 240)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compactProbeError(tt.err); got != tt.want {
				t.Fatalf("compactProbeError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeFilterProbesReturnFalseOnNonzeroExit(t *testing.T) {
	script := fakebin.Write(t, "probe ffmpeg", "printf '%s\\n' unavailable >&2\nexit 8\n")
	if probeNvidiaCUDAScale(script) {
		t.Fatal("failed CUDA scale probe reported usable")
	}
	if probeNvidiaCUDATonemap(script) {
		t.Fatal("failed CUDA tone-map probe reported usable")
	}
	if probeQSVScale(script) {
		t.Fatal("failed QSV scale probe reported usable")
	}
}

func TestCapabilityRecordersTolerateProbeFailuresAndNilMaps(t *testing.T) {
	script := fakebin.Write(t, "probe ffmpeg", "printf '%s\\n' failure >&2\nexit 3\n")
	caps := Capabilities{
		Filters:  map[string]bool{"scale_cuda": true},
		Encoders: map[string]bool{"h264_qsv": true},
	}

	caps.recordCLIOptions(script, []string{"readrate"})
	caps.recordFilterOptions(script, "scale_cuda", []string{"format"})
	caps.recordEncoderOptions(script, "h264_qsv", []string{"preset"})
	if caps.CLIOptions != nil || caps.FilterOptions != nil || caps.EncoderOptions != nil {
		t.Fatalf("failed help probes populated options: %#v", caps)
	}

	successScript := fakebin.Write(t, "probe ffmpeg", "printf '%s\\n' 'format value' '-preset value'\n")
	caps.recordFilterOptions(successScript, "scale_cuda", []string{"format"})
	caps.recordEncoderOptions(successScript, "h264_qsv", []string{"preset"})
	if !caps.FilterOptions["scale_cuda"]["format"] {
		t.Fatalf("nil filter option map was not initialized: %#v", caps.FilterOptions)
	}
	if !caps.EncoderOptions["h264_qsv"]["preset"] {
		t.Fatalf("nil encoder option map was not initialized: %#v", caps.EncoderOptions)
	}

	// Options are only recorded for filters and encoders the build lists, so
	// the help probe is never run for an unknown name.
	caps.recordFilterOptions(successScript, "missing", []string{"format"})
	caps.recordEncoderOptions(successScript, "missing", []string{"preset"})
	if caps.FilterOptions["missing"] != nil || caps.EncoderOptions["missing"] != nil {
		t.Fatalf("options were recorded for names the build does not list: %#v %#v", caps.FilterOptions, caps.EncoderOptions)
	}
}

// The recorders write the outer map key and the Supports* lookups read it. If
// the two normalize differently, a padded name is stored under a key nothing
// can ever look up, and the silent result is a hardware capability reported as
// missing. Every name below is padded and mixed-case on purpose.
func TestCapabilityRecordersKeyNamesTheSameWayLookupsRead(t *testing.T) {
	script := fakebin.Write(t, "probe ffmpeg", "printf '%s\\n' 'format value' '-preset value' '-dash value'\n")
	caps := Capabilities{
		Filters:  map[string]bool{"scale_cuda": true},
		Encoders: map[string]bool{"h264_qsv": true},
	}

	caps.recordFilterOptions(script, "  Scale_CUDA  ", []string{"  Format  "})
	caps.recordEncoderOptions(script, "  H264_QSV  ", []string{"  Preset  "})
	caps.recordMuxerFlags(script, "  MP4  ", []string{"  Dash  "})

	if !caps.SupportsFilterOption("scale_cuda", "format") {
		t.Fatalf("filter option was stored under an unreadable key: %#v", caps.FilterOptions)
	}
	if !caps.SupportsEncoderOption("h264_qsv", "preset") {
		t.Fatalf("encoder option was stored under an unreadable key: %#v", caps.EncoderOptions)
	}
	if !caps.SupportsMuxerFlag("mp4", "dash") {
		t.Fatalf("muxer flag was stored under an unreadable key: %#v", caps.MuxerFlags)
	}
}

func TestRunFFmpegProbeContextTimesOut(t *testing.T) {
	script := fakebin.Write(t, "slow ffmpeg", "exec sleep 5\n")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()

	_, err := runFFmpegProbeContext(ctx, script, "-version")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline", err)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("context-aware probe did not stop promptly: %s", time.Since(started))
	}
}

func TestRunFFmpegProbeContextReturnsOutputAndNonzeroError(t *testing.T) {
	script := fakebin.Write(t, "bad ffmpeg", "printf '%s\\n' diagnostic >&2\nexit 4\n")

	output, err := runFFmpegProbeContext(context.Background(), script, "-version")
	if err == nil {
		t.Fatal("expected nonzero probe error")
	}
	if !strings.Contains(output, "diagnostic") || !strings.Contains(err.Error(), "diagnostic") {
		t.Fatalf("probe output/error lost diagnostics: output=%q err=%v", output, err)
	}
}

// scale_vt was trusted on the strength of the encoder being listed, and the
// chain it was handed (system-memory frames, no upload) could not run on it.
// The probe runs the production chain itself, so a chain the hardware refuses
// costs the hardware tone-map rather than every HDR session on the device.
func TestProbeCapabilitiesGatesAppleToneMapOnTheProductionChain(t *testing.T) {
	appleFake := func(t *testing.T, logPath string, filters string, runtimeExit int) string {
		t.Helper()
		body := fakebin.CallLogLine(logPath) + `
if [ "$1" = "-encoders" ]; then
  printf '%s\n' ' V..... h264_videotoolbox VideoToolbox' ' V..... libx264 CPU'
  exit 0
fi
if [ "$1" = "-filters" ]; then
  printf '%s\n' ` + filters + `
  exit 0
fi
if [ "$1" = "-hwaccels" ]; then
  printf '%s\n' 'Hardware acceleration methods:' 'videotoolbox'
  exit 0
fi
case "$*" in
  *scale_vt*) exit ` + strconv.Itoa(runtimeExit) + ` ;;
esac
exit 0
`
		return fakebin.Write(t, "probe ffmpeg", body)
	}
	bothFilters := `' ... hwupload V->V' ' ... scale_vt V->V'`

	t.Run("a passing probe enables scale_vt", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "probes.log")
		caps := probeCapabilities(appleFake(t, logPath, bothFilters, 0), fakeFFmpegVersionBanner)
		if !caps.AppleVTTonemapRuntimeUsable || !caps.SupportsAppleVTTonemap() {
			t.Fatalf("scale_vt probe passed but the chain is not enabled: %#v", caps)
		}

		var probe string
		for _, line := range fakebin.ReadLog(t, logPath) {
			if strings.Contains(line, "scale_vt") {
				probe = line
			}
		}
		chain := hlsAppleVTTonemapFilter(appleProbeHeight)
		wantParts := []string{
			"-init_hw_device videotoolbox=" + hlsAppleVTDeviceName,
			"-filter_hw_device " + hlsAppleVTDeviceName,
			"color_trc=smpte2084",
			chain,
			"-c:v h264_videotoolbox",
		}
		for _, part := range wantParts {
			if !strings.Contains(probe, part) {
				t.Fatalf("scale_vt probe %q is missing %q", probe, part)
			}
		}
		taggedFirst := strings.Index(probe, "color_trc=smpte2084") < strings.Index(probe, chain)
		if !taggedFirst {
			t.Fatalf("scale_vt probe does not tag its frame as PQ before the chain: %s", probe)
		}
	})

	t.Run("a failing probe keeps the tone-map in software", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "probes.log")
		caps := probeCapabilities(appleFake(t, logPath, bothFilters, 1), fakeFFmpegVersionBanner)
		if caps.AppleVTTonemapRuntimeUsable || caps.SupportsAppleVTTonemap() {
			t.Fatalf("scale_vt enabled although its probe failed: %#v", caps)
		}
	})

	t.Run("a build without scale_vt is not probed", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "probes.log")
		caps := probeCapabilities(appleFake(t, logPath, `' ... hwupload V->V'`, 0), fakeFFmpegVersionBanner)
		if caps.AppleVTTonemapRuntimeUsable {
			t.Fatalf("scale_vt probe ran without the filter: %#v", caps)
		}
		ranProbe := slices.ContainsFunc(fakebin.ReadLog(t, logPath), func(line string) bool {
			return strings.Contains(line, "scale_vt")
		})
		if ranProbe {
			t.Fatal("scale_vt runtime probe ran although the build does not list the filter")
		}
	})
}
