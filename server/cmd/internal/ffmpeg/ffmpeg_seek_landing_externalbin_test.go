//go:build externalbin

package ffmpeg

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"igloo/cmd/internal/helpers"
)

// seekLandingSourceArgs encode 20 s at 25 fps with B-frames and keyframes
// only at 0, 5, 9.96 and 15 s, so where each seek lands is unambiguous.
func seekLandingSourceArgs(destination string, extra ...string) []string {
	args := []string{
		"-bf", "2",
		"-g", "1000", "-keyint_min", "1000", "-sc_threshold", "0",
		"-force_key_frames", "0,5,9.96,15",
		"-c:a", "aac", "-shortest",
	}
	args = append(args, extra...)
	return lavfiSampleArgs(destination,
		"testsrc2=size=160x90:rate=25:duration=20",
		"anullsrc=channel_layout=stereo:sample_rate=48000",
		args...)
}

// A start of 10 s is 40 ms after the 9.96 s keyframe. Matroska video with
// B-frames makes FFmpeg aim 3/23 s before the start, so the session copies
// from 5 s, a whole GOP earlier than "the last keyframe at or before 10".
func TestExternalSeekLandingMatroskaBFramesLandsAGOPEarly(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	source := filepath.Join(t.TempDir(), "b frames.mkv")
	runExternalFFmpegCommand(t, candidate.path, seekLandingSourceArgs(source)...)

	for _, tt := range []struct {
		startSec float64
		want     float64
	}{
		{startSec: 10, want: 5},
		{startSec: 11, want: 9.96},
	} {
		landing := measureSeekLanding(t, f, source, tt.startSec)
		if math.Abs(landing-tt.want) > 0.001 {
			t.Fatalf("start %v landed at %.3f s, want %.3f s", tt.startSec, landing, tt.want)
		}
	}

	// The session itself starts where the measurement said: its first segment
	// runs from the 5 s keyframe to the 9.96 s one (from 9.96 s it would end
	// at 15 s instead).
	params := basicHLSParams(t.TempDir())
	params.SourcePath = source
	params.Profile = helpers.HLS_PROFILE_REMUX
	params.CopyVideo = true
	params.CopyAudio = true
	params.StartSec = 10
	runExternalHLSAndWait(t, f, params)
	assertCompleteSequentialHLSOutput(t, params.OutDir)
	firstSegment := firstHLSSegmentDuration(t, params.OutDir)
	if math.Abs(firstSegment-(9.96-5)) > 0.03 {
		t.Fatalf("first segment lasts %.3f s, want %.3f s from the measured landing", firstSegment, 9.96-5)
	}
}

// An MP4 without an edit list seeks by decode time: the keyframe presented at
// 10.04 s is decoded at 9.96 s, so a start of 10 s lands after the start.
func TestExternalSeekLandingMP4CanLandAfterTheStart(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	source := filepath.Join(t.TempDir(), "no edit list.mp4")
	runExternalFFmpegCommand(t, candidate.path, seekLandingSourceArgs(source, "-use_editlist", "0")...)

	landing := measureSeekLanding(t, f, source, 10)
	if math.Abs(landing-10.04) > 0.001 {
		t.Fatalf("start 10 landed at %.3f s, want 10.040 s", landing)
	}
}

func measureSeekLanding(t *testing.T, f *ffmpeg, source string, startSec float64) float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	landing, err := f.SeekLanding(ctx, SeekLandingParams{SourcePath: source, StartSec: startSec})
	if err != nil {
		t.Fatalf("SeekLanding(%v): %v", startSec, err)
	}
	return landing
}

var firstExtinfPattern = regexp.MustCompile(`(?m)^#EXTINF:([0-9.]+),`)

func firstHLSSegmentDuration(t *testing.T, outDir string) float64 {
	t.Helper()
	playlist, err := os.ReadFile(filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME))
	if err != nil {
		t.Fatal(err)
	}
	match := firstExtinfPattern.FindSubmatch(playlist)
	if match == nil {
		t.Fatalf("playlist has no segments:\n%s", playlist)
	}
	duration, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return duration
}
