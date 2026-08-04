package ffmpeg

import (
	"strings"
	"testing"

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
