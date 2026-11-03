//go:build externalbin

package ffmpeg

import (
	"context"
	"encoding/binary"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

// Decode real HLS output: channel counts alone cannot detect a discarded LFE
// or surround channel. The isolated fixtures also pin which stereo side hears it.
func TestExternalFFmpegExplicitAudioSignals(t *testing.T) {
	candidate, err := resolveBinaryCandidate()
	if err != nil {
		t.Fatal(err)
	}
	prober, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}
	f := &ffmpeg{bin: candidate.path, capabilities: Capabilities{Probed: true}}
	cases := []struct {
		name, layout      string
		channels, ceiling int
		active, audible   []int
		// outLayout is the probed output layout when it is not the standard
		// one for the output channel count.
		outLayout string
	}{
		{"mono", "mono", 1, 6, []int{0}, []int{0}, ""},
		{"stereo", "stereo", 2, 6, []int{0, 1}, []int{0, 1}, ""},
		{"surround", "5.1", 6, 6, []int{0, 1, 2, 3, 4, 5}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"DTS surround", "5.1(side)", 6, 6, []int{0, 1, 2, 3, 4, 5}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"side surround", "5.1(side)", 6, 6, []int{0, 1, 2, 3, 4, 5}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"7.1", "7.1", 8, 6, []int{0, 1, 2, 3, 4, 5, 6, 7}, []int{0, 1, 2, 3, 4, 5}, ""},
		{"7.1 side left", "7.1", 8, 6, []int{6}, []int{4}, ""},
		{"7.1 back right", "7.1", 8, 6, []int{5}, []int{5}, ""},
		{"center downmix", "5.1", 6, 2, []int{2}, []int{0, 1}, ""},
		{"surround left downmix", "5.1", 6, 2, []int{4}, []int{0}, ""},
		{"surround right downmix", "5.1", 6, 2, []int{5}, []int{1}, ""},
		{"LFE downmix", "5.1", 6, 2, []int{3}, []int{0, 1}, ""},
		// Layouts kept as they are: -ac used to let the encoder choose its own
		// layout for the channel count, which dropped these LFE channels.
		{"2.1 LFE", "2.1", 3, 6, []int{2}, []int{2}, "2.1"},
		{"3.1 LFE", "3.1", 4, 6, []int{3}, []int{3}, "3.1"},
		{"4.1 LFE", "4.1", 5, 6, []int{3}, []int{3}, "4.1"},
		{"3.0 center", "3.0", 3, 6, []int{2}, []int{2}, "3.0"},
		{"4.0 back center", "4.0", 4, 6, []int{3}, []int{3}, "4.0"},
		{"2.1 LFE downmix", "2.1", 3, 2, []int{2}, []int{0, 1}, ""},
		// The encoders reject 6.0, so it is remixed to 5.1 with its back
		// center spread over both surrounds.
		{"6.0 back center", "6.0", 6, 6, []int{3}, []int{4, 5}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expressions := make([]string, tc.channels)
			for i := range expressions {
				expressions[i] = "0"
			}
			for _, i := range tc.active {
				expressions[i] = "0.1*sin(2*PI*100*t)"
			}
			source := filepath.Join(t.TempDir(), "signal.mkv")
			// WavPack records the channel mask in Matroska; PCM does not, so a
			// PCM source would decode as FFmpeg's guess for its channel count
			// (2.1 for any three channels) rather than the layout under test.
			// The tone has a duration because the WavPack encoder fails with
			// ENOSPC when -shortest cuts an endless source.
			sourceCodec := "wavpack"
			if tc.name == "DTS surround" {
				sourceCodec = "dca"
			}
			runExternalFFmpegCommand(t, candidate.path, lavfiSampleArgs(source,
				"testsrc2=size=160x90:rate=24:duration=3.2",
				"aevalsrc="+strings.Join(expressions, "|")+":s=44100:d=3.2:c="+tc.layout,
				"-c:a", sourceCodec, "-strict", "experimental", "-g", "24", "-shortest")...)
			for _, codec := range []helpers.HLSAudioCodec{helpers.HLSAudioCodecAC3, helpers.HLSAudioCodecEAC3} {
				t.Run(string(codec), func(t *testing.T) {
					params := basicHLSParams(t.TempDir())
					params.SourcePath = source
					params.CopyVideo = true
					params.Profile = helpers.HLS_PROFILE_REMUX
					params.StartSec = 1
					params.AudioProfile = resolveTestAudioProfile(codec, tc.ceiling, tc.channels, tc.layout)
					runExternalHLSAndWait(t, f, params)
					assertCompleteSequentialHLSOutput(t, params.OutDir)
					playlist := filepath.Join(params.OutDir, helpers.HLS_PLAYLIST_FILENAME)
					audio := probeExternalStream(t, prober, playlist, "audio")
					channels := min(tc.channels, tc.ceiling)
					layout := tc.outLayout
					if layout == "" {
						layout = helpers.HLSDefaultChannelLayoutName(channels)
						if channels == 6 {
							layout = "5.1(side)"
						}
					}
					if audio.CodecName != string(codec) || audio.Channels != channels || audio.ChannelLayout != layout || audio.SampleRate != "48000" {
						t.Fatalf("unexpected audio: %+v", audio)
					}
					ctx, cancel := context.WithTimeout(context.Background(), externalFFmpegIntegrationTimeout)
					defer cancel()
					cmd := exec.CommandContext(ctx, candidate.path, "-v", "error", "-i", playlist, "-map", "0:a:0", "-f", "f32le", "-c:a", "pcm_f32le", "pipe:1")
					pcm, err := cmd.Output()
					if err != nil {
						t.Fatalf("decode: %v", err)
					}
					frames := len(pcm) / (4 * channels)
					if frames < 48000 {
						t.Fatalf("only %d decoded frames after nonzero start", frames)
					}
					for ch := range channels {
						var sum float64
						for frame := range frames {
							offset := (frame*channels + ch) * 4
							sample := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[offset : offset+4])))
							sum += sample * sample
						}
						rms := math.Sqrt(sum / float64(frames))
						audible := false
						for _, want := range tc.audible {
							if ch == want {
								audible = true
							}
						}
						if (audible && rms < 0.005) || (!audible && rms > 0.001) || math.IsNaN(rms) {
							t.Errorf("%s channel %d RMS = %g, audible = %t", codec, ch, rms, audible)
						}
					}
				})
			}
		})
	}
}
