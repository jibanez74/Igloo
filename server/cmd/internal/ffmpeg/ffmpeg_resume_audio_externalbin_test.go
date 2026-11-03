//go:build externalbin

package ffmpeg

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
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

// A source whose audio starts 10 s after its video had no Dolby packet when
// the first 4 s segment closed, so the fMP4 muxer wrote init.mp4 without dec3
// or mvex. Encoded audio is padded with silence back to the video's start,
// keeping the tone where the source put it.
func TestExternalFFmpegLateSourceAudioWritesDolbyInit(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}

	const audioDelay = 10.0
	source := filepath.Join(t.TempDir(), "late-audio.mkv")
	runExternalFFmpegCommand(t, candidate.path,
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=160x90:rate=24:duration=20",
		"-itsoffset", fmt.Sprintf("%g", audioDelay),
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=10",
		"-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "48",
		"-c:a", "ac3", source)

	for _, codec := range []helpers.HLSAudioCodec{helpers.HLSAudioCodecEAC3, helpers.HLSAudioCodecAC3} {
		for _, start := range []struct {
			name      string
			sec       float64
			copyVideo bool
		}{
			{name: "remux from zero", copyVideo: true},
			{name: "transcode seek", sec: 3},
		} {
			t.Run(string(codec)+" "+start.name, func(t *testing.T) {
				params := basicHLSParams(t.TempDir())
				params.SourcePath = source
				params.StartSec = start.sec
				if start.copyVideo {
					params.Profile = helpers.HLS_PROFILE_REMUX
					params.CopyVideo = true
				}
				params.AudioProfile = resolveTestAudioProfile(codec, 2, 1, "mono")
				runExternalHLSAndWait(t, f, params)
				assertCompleteSequentialHLSOutput(t, params.OutDir)

				init, err := os.ReadFile(filepath.Join(params.OutDir, helpers.HLS_INIT_FILENAME))
				if err != nil {
					t.Fatal(err)
				}
				if !mp4HasBoxPath(init, "moov", "mvex") {
					t.Fatalf("init.mp4 has no moov/mvex (%d bytes)", len(init))
				}
				// The codec configuration box sits under stsd, a full box
				// mp4HasBoxPath cannot walk into, so search the bytes.
				configBox := "dac3"
				if codec == helpers.HLSAudioCodecEAC3 {
					configBox = "dec3"
				}
				if !bytes.Contains(init, []byte(configBox)) {
					t.Fatalf("init.mp4 has no %s box (%d bytes)", configBox, len(init))
				}

				playlist := filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME)
				first := firstPacketSeconds(t, candidate.path, playlist)
				video, audio := first[0], first[1]
				if math.Abs(audio-video) > 0.2 {
					t.Fatalf("padded audio starts at %.3f s but video at %.3f s", audio, video)
				}
				wantTone := audioDelay - start.sec
				if tone := firstSoundSeconds(t, candidate.path, playlist); math.Abs(tone-wantTone) > 0.1 {
					t.Fatalf("tone starts at %.3f s, want %.3f s", tone, wantTone)
				}
			})
		}
	}
}

// The start pad must leave a source whose audio starts with its video
// untouched: the same audio packets as the unpadded filter.
func TestExternalFFmpegAudioStartPadLeavesAlignedAudio(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}

	source := filepath.Join(t.TempDir(), "aligned.mkv")
	runExternalFFmpegCommand(t, candidate.path, lavfiSampleArgs(source,
		"testsrc2=size=160x90:rate=24:duration=12",
		"sine=frequency=440:sample_rate=48000:duration=12",
		"-g", "48", "-c:a", "ac3", "-shortest")...)

	for _, start := range []float64{0, 3.4} {
		t.Run(fmt.Sprintf("start %g", start), func(t *testing.T) {
			params := basicHLSParams(t.TempDir())
			params.SourcePath = source
			params.StartSec = start
			params.AudioProfile = resolveTestAudioProfile(helpers.HLSAudioCodecEAC3, 6, 2, "stereo")
			runExternalHLSAndWait(t, f, params)

			unpadded := params
			unpadded.OutDir = t.TempDir()
			args := hlsArgs(t, unpadded)
			filter := slices.Index(args, "-af") + 1
			plain, found := strings.CutSuffix(args[filter], ":"+hlsAudioStartPadOptions)
			if !found {
				t.Fatalf("-af %q has no start pad", args[filter])
			}
			args[filter] = plain
			runExternalFFmpegCommand(t, candidate.path, args...)

			got := audioFramecrc(t, candidate.path, filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME))
			want := audioFramecrc(t, candidate.path, filepath.Join(unpadded.OutDir, helpers.HLS_PLAYLIST_FILENAME))
			if got != want {
				t.Fatalf("padding changed aligned audio:\npadded:\n%s\nunpadded:\n%s", got, want)
			}
		})
	}
}

// audioFramecrc lists the HLS output's audio packets with their checksums.
func audioFramecrc(t *testing.T, binary string, playlist string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-v", "error", "-i", playlist, "-map", "0:a", "-c", "copy", "-f", "framecrc", "-").Output()
	if err != nil {
		t.Fatalf("framecrc: %v", err)
	}
	return string(out)
}

// firstSoundSeconds returns where the first silence of the HLS output's audio
// ends, which for a tone after leading silence is where the tone begins.
func firstSoundSeconds(t *testing.T, binary string, playlist string) float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "-hide_banner", "-nostats", "-i", playlist,
		"-map", "0:a", "-af", "silencedetect=n=-40dB:d=0.5", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("silencedetect: %v: %s", err, out)
	}
	match := regexp.MustCompile(`silence_end: ([0-9.]+)`).FindSubmatch(out)
	if match == nil {
		t.Fatalf("no leading silence in the audio:\n%s", out)
	}
	seconds, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return seconds
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
