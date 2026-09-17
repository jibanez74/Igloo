//go:build externalbin

package keyframeindex_test

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/keyframeindex"
)

const integrationTimeout = 60 * time.Second

// TestExtractMatchesFfprobeGroundTruth generates real mkv and mp4 files with
// the host ffmpeg and verifies the container-index extraction agrees with an
// ffprobe packet walk — the same ground truth the HLS session fallback uses.
func TestExtractMatchesFfprobeGroundTruth(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("host ffmpeg not on PATH")
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("host ffprobe not on PATH")
	}

	workspace := t.TempDir()

	for _, tc := range []struct {
		container string
		fileName  string
		extraArgs []string
	}{
		{container: "mkv", fileName: "sample.mkv"},
		// -movflags +faststart also exercises a front-positioned moov after
		// the second-pass rewrite.
		{container: "mp4", fileName: "sample.mp4", extraArgs: []string{"-movflags", "+faststart"}},
	} {
		t.Run(tc.container, func(t *testing.T) {
			sourcePath := filepath.Join(workspace, tc.fileName)
			generateSample(t, ffmpegPath, sourcePath, tc.extraArgs)

			truth := ffprobeKeyframes(t, ffprobePath, sourcePath)
			if len(truth) < 3 {
				t.Fatalf("ffprobe found only %d keyframes; generation is broken", len(truth))
			}

			file, err := os.Open(sourcePath)
			if err != nil {
				t.Fatalf("open sample: %v", err)
			}
			defer file.Close()
			stat, err := file.Stat()
			if err != nil {
				t.Fatalf("stat sample: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
			defer cancel()
			idx, err := keyframeindex.Extract(ctx, file, stat.Size(), tc.container)
			if err != nil {
				t.Fatalf("Extract returned error: %v", err)
			}

			if len(idx.KeyframeSec) != len(truth) {
				t.Fatalf("keyframe count = %d, ffprobe ground truth has %d\nindex: %v\ntruth: %v",
					len(idx.KeyframeSec), len(truth), idx.KeyframeSec, truth)
			}
			for i := range truth {
				if math.Abs(idx.KeyframeSec[i]-truth[i]) > 0.001 {
					t.Fatalf("keyframe[%d] = %.6f, ffprobe says %.6f", i, idx.KeyframeSec[i], truth[i])
				}
			}
			if math.Abs(idx.DurationSec-30) > 0.5 {
				t.Fatalf("DurationSec = %f, want ~30", idx.DurationSec)
			}
		})
	}
}

// generateSample renders 30 s of testsrc2 as H.264 with B-frames (forcing a
// ctts table) and keyframes pinned to 4 s boundaries.
func generateSample(t *testing.T, ffmpegPath, outPath string, extraArgs []string) {
	t.Helper()

	args := []string{
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=duration=30:size=320x180:rate=24",
		"-c:v", "libx264", "-preset", "ultrafast", "-bf", "2",
		"-force_key_frames", "expr:gte(t,n_forced*4)",
	}
	args = append(args, extraArgs...)
	args = append(args, outPath)

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffmpegPath, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("generate sample: %v\n%s", err, out)
	}
}

// ffprobeKeyframes runs the packet-walk ground truth: every video packet's
// pts_time whose flags start with K (newer ffprobe appends extra characters).
func ffprobeKeyframes(t *testing.T, ffprobePath, sourcePath string) []float64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "packet=pts_time,flags",
		"-of", "csv=p=0",
		sourcePath,
	).Output()
	if err != nil {
		t.Fatalf("ffprobe ground truth: %v", err)
	}

	keyframes := make([]float64, 0, 16)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(strings.TrimSpace(line), ",")
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "K") {
			continue
		}
		pts, parseErr := strconv.ParseFloat(fields[0], 64)
		if parseErr != nil {
			continue
		}
		keyframes = append(keyframes, pts)
	}
	sort.Float64s(keyframes)
	return keyframes
}
