package ffmpeg

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
	"igloo/cmd/internal/helpers"
)

const testProcessTimeout = 10 * time.Second

type hlsExitResult struct {
	exitErr    error
	stderrTail []string
}

// --- fake FFmpeg processes ---

func writeFakeFFmpeg(t *testing.T, name string, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	contents := "#!/bin/sh\nset -eu\n" + body
	if !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	err := os.WriteFile(path, []byte(contents), 0755)
	if err != nil {
		t.Fatalf("write fake FFmpeg: %v", err)
	}
	return path
}

// appendInvocationLog returns a shell prologue that appends one line per
// invocation holding the whole argument list, for fakes called several times.
func appendInvocationLog(logPath string) string {
	return "printf '%s\\n' \"$*\" >> " + formatShellPath(logPath) + "\n"
}

// writeArgumentLog returns a shell prologue that writes one line per argument,
// for fakes called once whose individual arguments are asserted.
func writeArgumentLog(logPath string) string {
	return "printf '%s\\n' \"$@\" > " + formatShellPath(logPath) + "\n"
}

func readArgumentLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read argument log: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func formatShellPath(path string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(path, "'", "'\\''"))
}

// --- RunHLS drivers ---

// startFakeHLS launches params and returns the channel exit callbacks land on,
// so callers can also assert that no second callback arrives.
func startFakeHLS(t *testing.T, f *ffmpeg, params HLSParams) chan hlsExitResult {
	t.Helper()
	results := make(chan hlsExitResult, 2)
	_, err := f.RunHLS(context.Background(), params, func(exitErr error, stderrTail []string) {
		results <- hlsExitResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}
	return results
}

func runFakeHLS(t *testing.T, f *ffmpeg, params HLSParams) hlsExitResult {
	t.Helper()
	return waitForHLSExit(t, startFakeHLS(t, f, params))
}

func waitForHLSExit(t *testing.T, results <-chan hlsExitResult) hlsExitResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testProcessTimeout)
	defer cancel()
	select {
	case result := <-results:
		return result
	case <-ctx.Done():
		t.Fatal("timed out waiting for FFmpeg exit callback")
		return hlsExitResult{}
	}
}

func waitForCommandExit(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	deadline := time.Now().Add(testProcessTimeout)
	for time.Now().Before(deadline) {
		err := cmd.Process.Signal(syscall.Signal(0))
		if errors.Is(err, os.ErrProcessDone) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for FFmpeg process to exit")
}

// --- FFmpeg argument assertions ---

func requireArgumentValue(t *testing.T, args []string, flag string, want string) {
	t.Helper()
	index := slices.Index(args, flag)
	if index < 0 {
		t.Fatalf("missing argument %q in %v", flag, args)
	}
	if index+1 >= len(args) {
		t.Fatalf("argument %q has no value in %v", flag, args)
	}
	if args[index+1] != want {
		t.Fatalf("%s = %q, want %q", flag, args[index+1], want)
	}
}

func requireArgumentBefore(t *testing.T, args []string, first string, second string) {
	t.Helper()
	firstIndex := slices.Index(args, first)
	secondIndex := slices.Index(args, second)
	if firstIndex < 0 || secondIndex < 0 {
		t.Fatalf("expected %q and %q in %v", first, second, args)
	}
	if firstIndex >= secondIndex {
		t.Fatalf("argument %q at %d must precede %q at %d", first, firstIndex, second, secondIndex)
	}
}

// requireArgSubstrings asserts that every want appears in the joined argument
// list and no notWant does. notFlags are matched against whole arguments, for
// flags whose name is a substring of another argument or value.
func requireArgSubstrings(t *testing.T, args []string, want []string, notWant []string, notFlags []string) {
	t.Helper()
	joined := strings.Join(args, " ")
	for _, fragment := range want {
		if !strings.Contains(joined, fragment) {
			t.Errorf("missing %q in args: %s", fragment, joined)
		}
	}
	for _, fragment := range notWant {
		if strings.Contains(joined, fragment) {
			t.Errorf("unexpected %q in args: %s", fragment, joined)
		}
	}
	for _, flag := range notFlags {
		if slices.Contains(args, flag) {
			t.Errorf("unexpected flag %q in args: %s", flag, joined)
		}
	}
}

// --- HLS parameter and capability fixtures ---

func basicHLSParams(outDir string) HLSParams {
	return HLSParams{
		SourcePath:       "/tmp/source file.mkv",
		OutDir:           outDir,
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         "cpu",
		Capabilities:     Capabilities{Probed: true},
	}
}

func hlsArgs(t *testing.T, p HLSParams) []string {
	t.Helper()
	args, err := buildHLSArgs(p)
	if err != nil {
		t.Fatalf("buildHLSArgs: %v", err)
	}
	return args
}

func hlsTestCapabilitiesForDevice(device string) Capabilities {
	caps := Capabilities{
		Probed:                 true,
		Encoders:               map[string]bool{},
		Filters:                map[string]bool{},
		HWAccels:               map[string]bool{},
		FilterOptions:          map[string]map[string]bool{},
		EncoderOptions:         map[string]map[string]bool{},
		H264NVENCRuntimeUsable: true,
	}

	switch device {
	case helpers.HARDWARE_ACCELERATION_DEVICE_APPLE:
		caps.Encoders["h264_videotoolbox"] = true
	case helpers.HARDWARE_ACCELERATION_DEVICE_INTEL:
		caps.Encoders["h264_qsv"] = true
		caps.HWAccels["qsv"] = true
		caps.H264QSVRuntimeUsable = true
		caps.EncoderOptions["h264_qsv"] = map[string]bool{
			"look_ahead": true,
			"forced_idr": true,
			"preset":     true,
		}
	case helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA:
		caps.Encoders["h264_nvenc"] = true
		caps.HWAccels["cuda"] = true
		caps.Filters["hwupload"] = true
		caps.Filters["scale_cuda"] = true
		caps.FilterOptions["scale_cuda"] = map[string]bool{"format": true}
	}

	return caps
}

func hlsTestIntelQSVScaleCapabilities() Capabilities {
	caps := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_INTEL)
	caps.Filters["scale_qsv"] = true
	caps.FilterOptions["scale_qsv"] = map[string]bool{"format": true}
	caps.QSVScaleRuntimeUsable = true
	return caps
}

func hlsTestNvidiaCapabilities(tonemap bool) Capabilities {
	caps := hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA)
	caps.NvidiaCUDAScaleRuntimeUsable = true
	if tonemap {
		caps.Filters["tonemap_cuda"] = true
		caps.FilterOptions["tonemap_cuda"] = map[string]bool{
			"format":  true,
			"p":       true,
			"t":       true,
			"m":       true,
			"tonemap": true,
			"desat":   true,
		}
		caps.NvidiaCUDATonemapRuntimeUsable = true
	}
	return caps
}

// --- fMP4 fixture construction and mutation ---

// readTestBox builds a single box and parses it back, which is the setup every
// parseTFHD/parseTRUN table case needs.
func readTestBox(t *testing.T, typ string, payload []byte) ([]byte, mp4Box) {
	t.Helper()
	data := fmp4testutil.Box(typ, payload)
	box, _, err := readBox(data, 0, len(data))
	if err != nil {
		t.Fatalf("read %s box: %v", typ, err)
	}
	return data, box
}

func appendNALForTest(destination []byte, width int, payload []byte) []byte {
	size := uint32(len(payload))
	length := make([]byte, width)
	switch width {
	case 1:
		length[0] = byte(size)
	case 2:
		binary.BigEndian.PutUint16(length, uint16(size))
	case 3:
		length[0] = byte(size >> 16)
		length[1] = byte(size >> 8)
		length[2] = byte(size)
	case 4:
		binary.BigEndian.PutUint32(length, size)
	}
	destination = append(destination, length...)
	return append(destination, payload...)
}

func updateTestBoxSize(data []byte, start int, size int) {
	binary.BigEndian.PutUint32(data[start:start+4], uint32(size))
}

// firstBoxInTrafForTest resolves moof -> traf -> typ in a generated segment.
func firstBoxInTrafForTest(t *testing.T, segment []byte, typ string) mp4Box {
	t.Helper()
	moof, found, err := findDirectChildBox(segment, 0, len(segment), "moof")
	if err != nil || !found {
		t.Fatalf("find moof: found=%v err=%v", found, err)
	}
	traf, found, err := findDirectChildBox(segment, moof.PayloadStart, moof.End, "traf")
	if err != nil || !found {
		t.Fatalf("find traf: found=%v err=%v", found, err)
	}
	box, found, err := findDirectChildBox(segment, traf.PayloadStart, traf.End, typ)
	if err != nil || !found {
		t.Fatalf("find %s: found=%v err=%v", typ, found, err)
	}
	return box
}

// patchTrafFieldForTest overwrites a 32-bit field at payloadOffset inside the
// first moof/traf/typ box of a generated segment.
func patchTrafFieldForTest(t *testing.T, segment []byte, typ string, payloadOffset int, value uint32) {
	t.Helper()
	box := firstBoxInTrafForTest(t, segment, typ)
	start := box.PayloadStart + payloadOffset
	if start+4 > box.End {
		t.Fatalf("%s field at +%d exceeds box bounds", typ, payloadOffset)
	}
	binary.BigEndian.PutUint32(segment[start:start+4], value)
}

// Payload offsets of the fields the validator tests mutate. tfhd is a full box
// with the track ID first; trun follows its sample count with the data offset,
// then one size/flags pair per sample.
const (
	tfhdTrackIDOffset          = 4
	trunDataOffsetOffset       = 8
	trunFirstSampleSizeOffset  = 12
	trunFirstSampleFlagsOffset = 16
)

// writeRemuxFixtureFiles writes an init segment plus the supplied media
// segments under the names ValidateRemuxSafety expects.
func writeRemuxFixtureFiles(t *testing.T, dir string, initData []byte, segments ...[]byte) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, helpers.HLS_INIT_FILENAME), initData, 0644)
	if err != nil {
		t.Fatalf("write init segment: %v", err)
	}
	for i, segment := range segments {
		name := fmt.Sprintf(
			"%s%d%s",
			helpers.HLS_SEGMENT_FILENAME_PREFIX,
			i,
			helpers.HLS_SEGMENT_FILENAME_SUFFIX,
		)
		err = os.WriteFile(filepath.Join(dir, name), segment, 0644)
		if err != nil {
			t.Fatalf("write segment %d: %v", i, err)
		}
	}
}
