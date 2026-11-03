//go:build externalbin

package ffmpeg

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/helpers"
)

// A copy-video session that starts past zero copies video from the keyframe
// before the target. An accurate seek trimmed encoded audio to the target
// itself, so audio began up to a GOP after the video. When that gap outlasted
// the first segment (a Matroska file whose Cues skip keyframes, like the
// Captain Marvel remux resumed at 1790 s), the fMP4 muxer wrote init.mp4
// before any E-AC-3 packet was parsed: no dec3, no mvex, and Media3 failed to
// play the session. Encoded audio must start with the copied video.
func TestExternalFFmpegCopyVideoResumeStartsEncodedAudioWithVideo(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}

	// Keyframes only every 10 s, so a start at 5 s copies video from 0 s.
	source := filepath.Join(t.TempDir(), "long-gop.mkv")
	runExternalFFmpegCommand(t, candidate.path, lavfiSampleArgs(source,
		"testsrc2=size=160x90:rate=24:duration=20",
		"anullsrc=channel_layout=5.1(side):sample_rate=48000",
		"-g", "240", "-keyint_min", "240", "-sc_threshold", "0",
		"-c:a", "dca", "-strict", "experimental", "-shortest")...)

	for _, codec := range []helpers.HLSAudioCodec{helpers.HLSAudioCodecEAC3, helpers.HLSAudioCodecAC3} {
		t.Run(string(codec), func(t *testing.T) {
			params := basicHLSParams(t.TempDir())
			params.SourcePath = source
			params.Profile = helpers.HLS_PROFILE_REMUX
			params.CopyVideo = true
			params.StartSec = 5
			params.AudioProfile = resolveTestAudioProfile(codec, 6, 6, "5.1(side)")
			runExternalHLSAndWait(t, f, params)
			assertCompleteSequentialHLSOutput(t, params.OutDir)

			init, err := os.ReadFile(filepath.Join(params.OutDir, helpers.HLS_INIT_FILENAME))
			if err != nil {
				t.Fatal(err)
			}
			if !mp4HasBoxPath(init, "moov", "mvex") {
				t.Fatalf("init.mp4 has no moov/mvex (%d bytes)", len(init))
			}

			first := firstPacketSeconds(t, candidate.path, filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME))
			video, audio := first[0], first[1]
			if math.Abs(audio-video) > 0.2 {
				t.Fatalf("encoded audio starts at %.3f s but copied video at %.3f s", audio, video)
			}
		})
	}
}

// mp4HasBoxPath reports whether data holds the nested box path, for example
// moov then mvex, walking only the top level of each container on the way.
func mp4HasBoxPath(data []byte, path ...string) bool {
	if len(path) == 0 {
		return true
	}
	for len(data) >= 8 {
		size := uint64(binary.BigEndian.Uint32(data[:4]))
		header := uint64(8)
		switch size {
		case 0:
			size = uint64(len(data))
		case 1:
			if len(data) < 16 {
				return false
			}
			size = binary.BigEndian.Uint64(data[8:16])
			header = 16
		}
		if size < header || size > uint64(len(data)) {
			return false
		}
		if string(data[4:8]) == path[0] && mp4HasBoxPath(data[header:size], path[1:]...) {
			return true
		}
		data = data[size:]
	}
	return false
}

// firstPacketSeconds returns each stream's first packet timestamp in the HLS
// output, in seconds, from FFmpeg's framecrc listing (decode order, which for
// these streams starts with the earliest timestamp).
func firstPacketSeconds(t *testing.T, binary string, playlist string) map[int]float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-v", "error", "-i", playlist, "-map", "0", "-c", "copy", "-f", "framecrc", "-").Output()
	if err != nil {
		t.Fatalf("framecrc: %v", err)
	}
	first := framecrcFirstPTS(out)
	if len(first) < 2 {
		t.Fatalf("framecrc listed %d streams:\n%s", len(first), out)
	}
	return first
}
