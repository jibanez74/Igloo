package ffmpeg

import (
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestBuildHLSArgs_TranscodeAll(t *testing.T) {
	sourcePath := "/safe/source.mkv"
	outDir := t.TempDir()
	profile := helpers.HLS_PROFILE_1080P_4MBPS

	args, err := BuildHLSArgs(sourcePath, outDir, profile, 0, 1, helpers.HARDWARE_ACCELERATION_DEVICE_CPU, false, false)
	if err != nil {
		t.Fatalf("BuildHLSArgs: %v", err)
	}
	argStr := strings.Join(args, " ")

	for _, want := range []string{
		sourcePath, outDir, "fmp4", "event", "playlist.m3u8",
		"-map 0:0", "-map 0:1",
		"libx264", "-preset", "veryfast",
		"-c:a aac", "-ac", "2", "-b:a", "192k",
		"scale=-2:1080",
		"-avoid_negative_ts", "make_zero",
		"-fflags", "+genpts",
	} {
		if !strings.Contains(argStr, want) {
			t.Errorf("expected %q in args: %s", want, argStr)
		}
	}
}

func TestBuildHLSArgs_CopyAudioOnly(t *testing.T) {
	args, err := BuildHLSArgs("/s", t.TempDir(), helpers.HLS_PROFILE_1080P_4MBPS, 0, 0, "cpu", false, true)
	if err != nil {
		t.Fatalf("BuildHLSArgs: %v", err)
	}
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "libx264") {
		t.Error("video should be transcoded with libx264")
	}
	if !strings.Contains(argStr, "-c:a copy") {
		t.Error("audio should use copy when copyAudio=true")
	}
	if strings.Contains(argStr, "-b:a") {
		t.Error("should not set audio bitrate when copying")
	}
}

func TestBuildHLSArgs_CopyBoth(t *testing.T) {
	args, err := BuildHLSArgs("/s", t.TempDir(), helpers.HLS_PROFILE_720P_3MBPS, 0, 0, "cpu", true, true)
	if err != nil {
		t.Fatalf("BuildHLSArgs: %v", err)
	}
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-c:v copy") {
		t.Error("video should use copy")
	}
	if !strings.Contains(argStr, "-c:a copy") {
		t.Error("audio should use copy")
	}
	if strings.Contains(argStr, "libx264") || strings.Contains(argStr, "-hwaccel") {
		t.Error("should not use encoder or hwaccel when copying")
	}
}

func TestBuildHLSArgs_HWAccelBeforeInput(t *testing.T) {
	args, err := BuildHLSArgs("/s", t.TempDir(), helpers.HLS_PROFILE_720P_3MBPS, 0, 0, helpers.HARDWARE_ACCELERATION_DEVICE_APPLE, false, false)
	if err != nil {
		t.Fatalf("BuildHLSArgs: %v", err)
	}

	hwIdx := indexOf(args, "-hwaccel")
	iIdx := indexOf(args, "-i")
	if hwIdx < 0 {
		t.Fatal("-hwaccel flag missing for apple device")
	}
	if hwIdx >= iIdx {
		t.Errorf("-hwaccel (pos %d) must come before -i (pos %d)", hwIdx, iIdx)
	}
}

func TestBuildHLSArgs_Remux(t *testing.T) {
	args, err := BuildHLSArgs("/s", t.TempDir(), helpers.HLS_PROFILE_REMUX, 0, 0, "cpu", false, false)
	if err != nil {
		t.Fatalf("BuildHLSArgs: %v", err)
	}
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-c:v copy") {
		t.Error("remux must use -c:v copy")
	}
	if !strings.Contains(argStr, "-c:a aac") {
		t.Error("remux with copyAudio=false must transcode audio to AAC")
	}
	for _, forbidden := range []string{"libx264", "h264_videotoolbox", "h264_nvenc", "h264_qsv", "-hwaccel", "scale="} {
		if strings.Contains(argStr, forbidden) {
			t.Errorf("remux must not contain %q in args: %s", forbidden, argStr)
		}
	}
}

func TestBuildHLSArgs_RemuxCopyAudio(t *testing.T) {
	args, err := BuildHLSArgs("/s", t.TempDir(), helpers.HLS_PROFILE_REMUX, 0, 0, "cpu", false, true)
	if err != nil {
		t.Fatalf("BuildHLSArgs: %v", err)
	}
	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "-c:v copy") {
		t.Error("remux must use -c:v copy")
	}
	if !strings.Contains(argStr, "-c:a copy") {
		t.Error("remux with copyAudio=true must use -c:a copy")
	}
}

func TestBuildHLSArgs_InvalidProfile(t *testing.T) {
	_, err := BuildHLSArgs("/s", t.TempDir(), "4k_20mbps", 0, 0, "cpu", false, false)
	if err == nil {
		t.Error("expected error for disallowed profile")
	}
}

func TestBuildHLSArgs_SegmentFilenameInOutDir(t *testing.T) {
	outDir := t.TempDir()
	args, err := BuildHLSArgs("/s", outDir, helpers.HLS_PROFILE_1080P_4MBPS, 0, 0, "cpu", false, false)
	if err != nil {
		t.Fatalf("BuildHLSArgs: %v", err)
	}
	want := filepath.Join(outDir, "segment_%d.m4s")
	found := false
	for _, a := range args {
		if a == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -hls_segment_filename %q in args", want)
	}
}

func indexOf(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}
