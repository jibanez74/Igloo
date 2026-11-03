//go:build externalbin

package ffmpeg

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

func TestExternalSeekLandingRelativeToContainerStart(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	prober, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}

	for _, startPTS := range []int{0, 5} {
		t.Run("start PTS "+strconv.Itoa(startPTS), func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "five second GOP.mkv")
			generateLavfiSample(t, candidate.path, source,
				"testsrc2=size=160x90:rate=25:duration=20",
				"anullsrc=channel_layout=stereo:sample_rate=48000",
				"-an", "-bf", "0",
				"-g", "125", "-keyint_min", "125", "-sc_threshold", "0",
				"-output_ts_offset", strconv.Itoa(startPTS),
			)
			ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
			defer cancel()
			metadata, err := prober.GetMetadata(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			containerStart, err := strconv.ParseFloat(metadata.Format.StartTime, 64)
			if err != nil || math.Abs(containerStart-float64(startPTS)) > 0.001 {
				t.Fatalf("container start = %q (%v), want %d", metadata.Format.StartTime, err, startPTS)
			}

			landing := measureSeekLanding(t, f, source, 12)
			if math.Abs(landing-10) > 0.001 {
				t.Fatalf("start 12 landed at %.3f s relative to container start, want 10.000 s", landing)
			}
		})
	}
}

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
// Older FFmpeg muxers store the keyframes a frame-delay later (5.021, 9.981),
// so the expectations come from the file's own keyframes.
func TestExternalSeekLandingMatroskaBFramesLandsAGOPEarly(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	source := filepath.Join(t.TempDir(), "b frames.mkv")
	runExternalFFmpegCommand(t, candidate.path, seekLandingSourceArgs(source)...)
	keyframes := videoKeyframeSeconds(t, candidate.path, source)
	if len(keyframes) != 4 || keyframes[2] > 10 {
		t.Fatalf("keyframes at %v, want 4 with the third at or before 10 s", keyframes)
	}

	for _, tt := range []struct {
		startSec float64
		want     float64
	}{
		{startSec: 10, want: keyframes[1]},
		{startSec: 11, want: keyframes[2]},
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
	wantSegment := keyframes[2] - keyframes[1]
	if math.Abs(firstSegment-wantSegment) > 0.03 {
		t.Fatalf("first segment lasts %.3f s, want %.3f s from the measured landing", firstSegment, wantSegment)
	}
}

// videoKeyframeSeconds lists the presentation times of the source's video
// keyframes. framecrc flags every packet that is not a plain keyframe.
func videoKeyframeSeconds(t *testing.T, binary string, source string) []float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-v", "error", "-i", source, "-map", "0:v", "-c", "copy", "-f", "framecrc", "-").Output()
	if err != nil {
		t.Fatalf("framecrc: %v", err)
	}
	timeBase := 0.0
	var keyframes []float64
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSpace(line)
		if fraction, ok := strings.CutPrefix(line, "#tb 0: "); ok {
			num, den, _ := strings.Cut(fraction, "/")
			numerator, numErr := strconv.ParseFloat(num, 64)
			denominator, denErr := strconv.ParseFloat(den, 64)
			if numErr != nil || denErr != nil || denominator == 0 {
				t.Fatalf("bad time base %q", line)
			}
			timeBase = numerator / denominator
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.Contains(line, "F=0x") {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		pts, err := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		if err != nil {
			t.Fatalf("bad framecrc line %q", line)
		}
		keyframes = append(keyframes, pts*timeBase)
	}
	return keyframes
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
