package ffmpeg

import (
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestParseFFmpegNamedRows(t *testing.T) {
	output := `Encoders:
 V....D h264_nvenc           NVIDIA NVENC H.264 encoder
 V..... h264_qsv             H.264 QSV encoder
 ... scale_cuda        V->V       GPU accelerated video resizer
`
	found := parseFFmpegNamedRows(output)

	for _, name := range []string{"h264_nvenc", "h264_qsv", "scale_cuda"} {
		if !found[name] {
			t.Fatalf("expected %q in parsed rows: %#v", name, found)
		}
	}
}

func TestResolveHLSDeviceFallsBackForUnavailableNvidia(t *testing.T) {
	caps := Capabilities{
		Probed:                 true,
		Encoders:               map[string]bool{"h264_nvenc": true},
		H264NVENCRuntimeUsable: false,
		H264NVENCProbeError:    "no capable devices",
	}

	decision := ResolveHLSDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA, caps)
	if decision.Effective != helpers.HARDWARE_ACCELERATION_DEVICE_CPU {
		t.Fatalf("Effective = %q, want cpu", decision.Effective)
	}
	if !strings.Contains(decision.Reason, "runtime probe failed") || !strings.Contains(decision.Reason, "no capable devices") {
		t.Fatalf("unexpected fallback reason: %q", decision.Reason)
	}
}

func TestResolveHLSDeviceFallsBackForUnavailableIntelQSV(t *testing.T) {
	caps := Capabilities{
		Probed:               true,
		Encoders:             map[string]bool{"h264_qsv": true},
		H264QSVRuntimeUsable: false,
		H264QSVProbeError:    "no qsv device",
	}

	decision := ResolveHLSDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL, caps)
	if decision.Effective != helpers.HARDWARE_ACCELERATION_DEVICE_CPU {
		t.Fatalf("Effective = %q, want cpu", decision.Effective)
	}
	if !strings.Contains(decision.Reason, "runtime probe failed") || !strings.Contains(decision.Reason, "no qsv device") {
		t.Fatalf("unexpected fallback reason: %q", decision.Reason)
	}
}

func TestFFmpegFilterHelpHasOptionMatchesOptionName(t *testing.T) {
	output := `Filter tonemap_cuda
  Inputs:
     #0: default (video)
  tonemap_cuda AVOptions:
   format            <pix_fmt>    ..FV....... Output pixel format
   tonemap           <int>        ..FV....... Tonemap algorithm
`

	if !ffmpegHelpHasOption(output, "format") {
		t.Fatal("expected format option")
	}
	if ffmpegHelpHasOption(output, "p") {
		t.Fatal("single-letter option must not match arbitrary words in help output")
	}
}

func TestFFmpegHelpHasOptionMatchesDashedOptionName(t *testing.T) {
	output := `Encoder h264_qsv
  h264_qsv AVOptions:
   -preset           <int>        E..V....... Encoding preset
   -look_ahead       <boolean>    E..V....... Use lookahead
`

	if !ffmpegHelpHasOption(output, "look_ahead") {
		t.Fatal("expected look_ahead option")
	}
	if ffmpegHelpHasOption(output, "look") {
		t.Fatal("partial option name must not match")
	}
}

func TestSupportsNvidiaCUDAFiltersRequiresToneMapOptions(t *testing.T) {
	caps := Capabilities{
		Probed:                       true,
		Encoders:                     map[string]bool{"h264_nvenc": true},
		HWAccels:                     map[string]bool{"cuda": true},
		Filters:                      map[string]bool{"hwupload": true, "scale_cuda": true, "tonemap_cuda": true},
		FilterOptions:                map[string]map[string]bool{"scale_cuda": {"format": true}, "tonemap_cuda": {"format": true}},
		H264NVENCRuntimeUsable:       true,
		NvidiaCUDAScaleRuntimeUsable: true,
	}

	if !caps.SupportsNvidiaCUDAFilters(false) {
		t.Fatal("expected CUDA SDR scaling support")
	}
	if caps.SupportsNvidiaCUDAFilters(true) {
		t.Fatal("expected CUDA HDR tone-map support to require p/t/m/tonemap/desat options")
	}

	caps.FilterOptions["tonemap_cuda"] = map[string]bool{
		"format":  true,
		"p":       true,
		"t":       true,
		"m":       true,
		"tonemap": true,
		"desat":   true,
	}
	if caps.SupportsNvidiaCUDAFilters(true) {
		t.Fatal("expected CUDA HDR tone-map support to require a runtime probe")
	}
	caps.NvidiaCUDATonemapRuntimeUsable = true
	if !caps.SupportsNvidiaCUDAFilters(true) {
		t.Fatal("expected CUDA HDR tone-map support when required options are present")
	}
}

func TestSupportsNvidiaCUDAFiltersRequiresRuntimeScaleProbe(t *testing.T) {
	caps := Capabilities{
		Probed:                 true,
		Encoders:               map[string]bool{"h264_nvenc": true},
		HWAccels:               map[string]bool{"cuda": true},
		Filters:                map[string]bool{"hwupload": true, "scale_cuda": true},
		FilterOptions:          map[string]map[string]bool{"scale_cuda": {"format": true}},
		H264NVENCRuntimeUsable: true,
	}

	if caps.SupportsNvidiaCUDAFilters(false) {
		t.Fatal("expected CUDA scale support to require a runtime probe")
	}

	caps.NvidiaCUDAScaleRuntimeUsable = true
	if !caps.SupportsNvidiaCUDAFilters(false) {
		t.Fatal("expected CUDA scale support when static support and runtime probe are present")
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

	empty := Capabilities{}
	if empty.SupportsEncoder("x") || empty.SupportsFilter("x") || empty.SupportsHWAccel("x") {
		t.Fatal("nil capability maps reported support")
	}
	if empty.SupportsCLIOption("x") || empty.SupportsFilterOption("x", "y") {
		t.Fatal("nil option maps reported support")
	}
	if empty.SupportsEncoderOption("x", "y") {
		t.Fatal("nil encoder option map reported support")
	}
}

func TestCapabilityParsersHandleHeadersMalformedRowsAndCase(t *testing.T) {
	named := parseFFmpegNamedRows(`
Encoders:
 V..... H264_NVENC description
 -bad rejected
 X
 .. name=value malformed
 ... SCALE_CUDA V->V
`)
	if !named["h264_nvenc"] || !named["scale_cuda"] {
		t.Fatalf("parsed named rows = %#v", named)
	}
	if named["rejected"] || named["name=value"] {
		t.Fatalf("malformed rows were accepted: %#v", named)
	}

	hwaccels := parseFFmpegHWAccels("Hardware acceleration methods:\n CUDA \nQSV\n\n")
	if !hwaccels["cuda"] || !hwaccels["qsv"] || len(hwaccels) != 2 {
		t.Fatalf("parsed hardware accelerators = %#v", hwaccels)
	}
}

func TestFFmpegHelpHasOptionRejectsBlankAndMalformedLines(t *testing.T) {
	output := "\n   \n-readrate value\nreadrate_initial_burst value\n"
	if ffmpegHelpHasOption(output, "") {
		t.Fatal("blank option matched")
	}
	if !ffmpegHelpHasOption(output, " READRATE ") {
		t.Fatal("dashed option did not match after normalization")
	}
	if ffmpegHelpHasOption(output, "rate") {
		t.Fatal("partial option matched")
	}
}
