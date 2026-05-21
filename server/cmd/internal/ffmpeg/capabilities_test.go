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

func TestFFmpegFilterHelpHasOptionMatchesOptionName(t *testing.T) {
	output := `Filter tonemap_cuda
  Inputs:
     #0: default (video)
  tonemap_cuda AVOptions:
   format            <pix_fmt>    ..FV....... Output pixel format
   tonemap           <int>        ..FV....... Tonemap algorithm
`

	if !ffmpegFilterHelpHasOption(output, "format") {
		t.Fatal("expected format option")
	}
	if ffmpegFilterHelpHasOption(output, "p") {
		t.Fatal("single-letter option must not match arbitrary words in help output")
	}
}

func TestSupportsNvidiaCUDAFiltersRequiresToneMapOptions(t *testing.T) {
	caps := Capabilities{
		Probed:        true,
		HWAccels:      map[string]bool{"cuda": true},
		Filters:       map[string]bool{"scale_cuda": true, "tonemap_cuda": true},
		FilterOptions: map[string]map[string]bool{"scale_cuda": {"format": true}, "tonemap_cuda": {"format": true}},
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
	if !caps.SupportsNvidiaCUDAFilters(true) {
		t.Fatal("expected CUDA HDR tone-map support when required options are present")
	}
}
