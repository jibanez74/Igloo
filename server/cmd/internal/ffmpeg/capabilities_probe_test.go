package ffmpeg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fullCapabilityProbeFake(t *testing.T, runtimeExit int, logPath string) string {
	t.Helper()
	runtimeResult := "exit 0"
	if runtimeExit != 0 {
		runtimeResult = "printf '%0300d' 0 >&2\nexit " + strconv.Itoa(runtimeExit)
	}
	body := "printf '%s\\n' \"$*\" >> " + formatShellPath(logPath) + `
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
` + runtimeResult + "\n"
	return writeFakeFFmpeg(t, "probe ffmpeg", body)
}

func TestProbeCapabilitiesSuccessfulStaticAndRuntimeProbes(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "probes.log")
	script := fullCapabilityProbeFake(t, 0, logPath)

	caps := probeCapabilities(script)
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
	if !caps.SupportsEncoderOption("h264_qsv", "forced_idr") {
		t.Fatalf("encoder options were not orchestrated: %#v", caps.EncoderOptions)
	}
	if !caps.H264NVENCRuntimeUsable || !caps.NvidiaCUDAScaleRuntimeUsable {
		t.Fatalf("NVENC/CUDA runtime probes did not succeed: %#v", caps)
	}
	if !caps.NvidiaCUDATonemapRuntimeUsable || !caps.H264QSVRuntimeUsable || !caps.QSVScaleRuntimeUsable {
		t.Fatalf("tone-map/QSV runtime probes did not succeed: %#v", caps)
	}
}

func TestProbeCapabilitiesMissingPrerequisitesSuppressesFilterRuntimeProbes(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "probes.log")
	body := "printf '%s\\n' \"$*\" >> " + formatShellPath(logPath) + `
if [ "$1" = "-encoders" ]; then
  printf '%s\n' ' V..... h264_nvenc NVIDIA' ' V..... h264_qsv Intel'
fi
exit 0
`
	script := writeFakeFFmpeg(t, "probe ffmpeg", body)

	caps := probeCapabilities(script)
	if !caps.H264NVENCRuntimeUsable || !caps.H264QSVRuntimeUsable {
		t.Fatalf("encoder runtime probes should succeed in the fake: %#v", caps)
	}
	if caps.NvidiaCUDAScaleRuntimeUsable || caps.NvidiaCUDATonemapRuntimeUsable || caps.QSVScaleRuntimeUsable {
		t.Fatalf("filter runtime probes ran without prerequisites: %#v", caps)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read probe log: %v", err)
	}
	if strings.Contains(string(logData), "-init_hw_device") {
		t.Fatalf("hardware filter runtime probe was not suppressed:\n%s", logData)
	}
}

func TestProbeCapabilitiesRecordsBoundedEncoderDiagnostics(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "probes.log")
	script := fullCapabilityProbeFake(t, 7, logPath)

	caps := probeCapabilities(script)
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

	caps.recordFilterOptions(script, "missing", []string{"format"})
	caps.recordEncoderOptions(script, "missing", []string{"preset"})

	successScript := writeFakeFFmpeg(t, "probe ffmpeg", "printf '%s\\n' 'format value' '-preset value'\n")
	caps.recordFilterOptions(successScript, "scale_cuda", []string{"format"})
	caps.recordEncoderOptions(successScript, "h264_qsv", []string{"preset"})
	if !caps.FilterOptions["scale_cuda"]["format"] {
		t.Fatalf("nil filter option map was not initialized: %#v", caps.FilterOptions)
	}
	if !caps.EncoderOptions["h264_qsv"]["preset"] {
		t.Fatalf("nil encoder option map was not initialized: %#v", caps.EncoderOptions)
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

func TestCompactProbeError(t *testing.T) {
	if compactProbeError(nil) != "" {
		t.Fatal("nil error should compact to an empty string")
	}
	short := errors.New(" short diagnostic ")
	if compactProbeError(short) != "short diagnostic" {
		t.Fatalf("short diagnostic = %q", compactProbeError(short))
	}
	long := errors.New(strings.Repeat("x", 300))
	if len(compactProbeError(long)) != 240 {
		t.Fatalf("long diagnostic length = %d, want 240", len(compactProbeError(long)))
	}
}
