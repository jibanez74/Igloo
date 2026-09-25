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
)

// fakeFFmpegVersionBanner stands in for the `-version` output that
// initializeCandidate captures before handing it to probeCapabilities.
const fakeFFmpegVersionBanner = "ffmpeg version 7.0.2-Jellyfin Copyright (c) 2000-2024 the FFmpeg developers\n"

func fullCapabilityProbeFake(t *testing.T, runtimeExit int, logPath string) string {
	t.Helper()
	runtimeResult := "exit 0"
	if runtimeExit != 0 {
		runtimeResult = "printf '%0300d' 0 >&2\nexit " + strconv.Itoa(runtimeExit)
	}
	body := appendInvocationLog(logPath) + `
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
	return writeFakeFFmpeg(t, "probe ffmpeg", body)
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
	for _, line := range readArgumentLog(t, logPath) {
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
	script := writeFakeFFmpeg(t, "probe ffmpeg", writeArgumentLog(logPath)+"exit 0\n")

	if !probeNvidiaCUDATonemap(script) {
		t.Fatal("tone-map probe failed against a fake that accepts it")
	}
	args := readArgumentLog(t, logPath)
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
	body := appendInvocationLog(logPath) + `
if [ "$1" = "-encoders" ]; then
  printf '%s\n' ' V..... h264_nvenc NVIDIA' ' V..... h264_qsv Intel'
fi
exit 0
`
	script := writeFakeFFmpeg(t, "probe ffmpeg", body)

	caps := probeCapabilities(script, fakeFFmpegVersionBanner)
	if !caps.H264NVENCRuntimeUsable || !caps.H264QSVRuntimeUsable {
		t.Fatalf("encoder runtime probes should succeed in the fake: %#v", caps)
	}
	if caps.NvidiaCUDAScaleRuntimeUsable || caps.NvidiaCUDATonemapRuntimeUsable || caps.QSVScaleRuntimeUsable {
		t.Fatalf("filter runtime probes ran without prerequisites: %#v", caps)
	}

	invocations := readArgumentLog(t, logPath)
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
	script := writeFakeFFmpeg(t, "probe ffmpeg", "printf '%s\\n' unavailable >&2\nexit 8\n")
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
	script := writeFakeFFmpeg(t, "probe ffmpeg", "printf '%s\\n' failure >&2\nexit 3\n")
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

	successScript := writeFakeFFmpeg(t, "probe ffmpeg", "printf '%s\\n' 'format value' '-preset value'\n")
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
	script := writeFakeFFmpeg(t, "probe ffmpeg", "printf '%s\\n' 'format value' '-preset value' '-dash value'\n")
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
	script := writeFakeFFmpeg(t, "slow ffmpeg", "exec sleep 5\n")
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
	script := writeFakeFFmpeg(t, "bad ffmpeg", "printf '%s\\n' diagnostic >&2\nexit 4\n")

	output, err := runFFmpegProbeContext(context.Background(), script, "-version")
	if err == nil {
		t.Fatal("expected nonzero probe error")
	}
	if !strings.Contains(output, "diagnostic") || !strings.Contains(err.Error(), "diagnostic") {
		t.Fatalf("probe output/error lost diagnostics: output=%q err=%v", output, err)
	}
}
