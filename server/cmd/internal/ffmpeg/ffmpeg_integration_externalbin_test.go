//go:build externalbin

package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

const externalFFmpegIntegrationTimeout = 45 * time.Second

func TestExternalFFmpegCPUHLSRemuxAndSubtitles(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatalf("resolve external FFmpeg: %v", err)
	}
	workspace := filepath.Join(t.TempDir(), "media workspace")
	err = os.Mkdir(workspace, 0755)
	if err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	sourcePath := filepath.Join(workspace, "tiny source.mp4")
	generateTinyH264AACSource(t, candidate.path, sourcePath)

	f := &ffmpeg{
		bin:          candidate.path,
		capabilities: Capabilities{Probed: true},
	}

	t.Run("CPU transcode", func(t *testing.T) {
		outDir := filepath.Join(workspace, "CPU HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir CPU output: %v", err)
		}
		params := HLSParams{
			SourcePath:       sourcePath,
			OutDir:           outDir,
			Profile:          helpers.HLS_PROFILE_720P_3MBPS,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			CopyAudio:        true,
			SourceFrameRate:  24,
			Capabilities:     Capabilities{Probed: true},
		}
		runExternalHLSAndWait(t, f, params)
		segments := assertCompleteSequentialHLSOutput(t, outDir)
		if len(segments) < 2 {
			t.Fatalf("CPU transcode produced %d segments, want at least 2", len(segments))
		}
	})

	t.Run("H264 AAC remux", func(t *testing.T) {
		outDir := filepath.Join(workspace, "remux HLS")
		err := os.Mkdir(outDir, 0755)
		if err != nil {
			t.Fatalf("mkdir remux output: %v", err)
		}
		params := HLSParams{
			SourcePath:       sourcePath,
			OutDir:           outDir,
			Profile:          helpers.HLS_PROFILE_REMUX,
			VideoStreamIndex: 0,
			AudioStreamIndex: 1,
			HWDevice:         helpers.HARDWARE_ACCELERATION_DEVICE_CPU,
			CopyVideo:        true,
			CopyAudio:        true,
			Capabilities:     Capabilities{Probed: true},
		}
		runExternalHLSAndWait(t, f, params)
		segments := assertCompleteSequentialHLSOutput(t, outDir)
		summary, validationErr := ValidateRemuxSafety(outDir, len(segments))
		if validationErr != nil {
			t.Fatalf("ValidateRemuxSafety: %v", validationErr)
		}
		if summary.CheckedSegments != len(segments) || summary.CheckedSyncSamples < len(segments) {
			t.Fatalf("remux validation summary = %#v, segments=%d", summary, len(segments))
		}
	})

	t.Run("subtitle conversion", func(t *testing.T) {
		subtitlePath := filepath.Join(workspace, "subtitle source.srt")
		subtitle := "1\n00:00:00,000 --> 00:00:01,500\nIgloo subtitle\n"
		err := os.WriteFile(subtitlePath, []byte(subtitle), 0644)
		if err != nil {
			t.Fatalf("write subtitle source: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
		defer cancel()
		output, extractErr := f.ExtractSubtitleAsWebVTT(ctx, subtitlePath, 0)
		if extractErr != nil {
			t.Fatalf("ExtractSubtitleAsWebVTT: %v", extractErr)
		}
		if !strings.HasPrefix(string(output), "WEBVTT") || !strings.Contains(string(output), "Igloo subtitle") {
			t.Fatalf("real subtitle output = %q", output)
		}
	})
}

func generateTinyH264AACSource(t *testing.T, binary string, destination string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	args := []string{
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=5.2",
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=5.2",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-g", "24", "-keyint_min", "24", "-sc_threshold", "0",
		"-c:a", "aac", "-shortest", "-movflags", "+faststart",
		destination,
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate H.264/AAC source: %v: %s", err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat generated source: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("generated source is empty")
	}
}

func runExternalHLSAndWait(t *testing.T, f *ffmpeg, params HLSParams) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	results := make(chan hlsExitResult, 1)
	_, err := f.RunHLS(ctx, params, func(exitErr error, stderrTail []string) {
		results <- hlsExitResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}

	select {
	case result := <-results:
		if result.exitErr != nil {
			t.Fatalf("external FFmpeg exit: %v\nstderr tail:\n%s", result.exitErr, strings.Join(result.stderrTail, "\n"))
		}
	case <-ctx.Done():
		t.Fatalf("external FFmpeg HLS timed out: %v", ctx.Err())
	}
}

func assertCompleteSequentialHLSOutput(t *testing.T, outDir string) []string {
	t.Helper()
	playlistPath := filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME)
	playlistData, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatalf("read HLS playlist: %v", err)
	}
	playlist := string(playlistData)
	if !strings.Contains(playlist, "#EXT-X-ENDLIST") {
		t.Fatalf("playlist is incomplete:\n%s", playlist)
	}
	wantMap := fmt.Sprintf("#EXT-X-MAP:URI=\"%s\"", helpers.HLS_INIT_FILENAME)
	if !strings.Contains(playlist, wantMap) {
		t.Fatalf("playlist missing %q:\n%s", wantMap, playlist)
	}
	assertNonemptyFile(t, filepath.Join(outDir, helpers.HLS_INIT_FILENAME))

	segmentPattern := regexp.MustCompile(`(?m)^segment_([0-9]+)\.m4s$`)
	matches := segmentPattern.FindAllStringSubmatch(playlist, -1)
	if len(matches) == 0 {
		t.Fatalf("playlist has no media segments:\n%s", playlist)
	}
	segments := make([]string, 0, len(matches))
	for index, match := range matches {
		sequence, conversionErr := strconv.Atoi(match[1])
		if conversionErr != nil {
			t.Fatalf("parse segment sequence %q: %v", match[1], conversionErr)
		}
		if sequence != index {
			t.Fatalf("segment sequence = %d at position %d, want %d", sequence, index, index)
		}
		segments = append(segments, match[0])
		assertNonemptyFile(t, filepath.Join(outDir, match[0]))
	}
	return segments
}

func assertNonemptyFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("file %s is empty", path)
	}
}
