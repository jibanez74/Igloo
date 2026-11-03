package ffmpeg

import (
	"context"
	"slices"
	"testing"

	"igloo/cmd/internal/helpers"
)

// The measurement is only right if FFmpeg opens and seeks the source exactly
// as the session does: probe size and analysis duration decide whether the
// B-frame rule applies, and -noaccurate_seek and -ss decide the seek itself.
func TestSeekLandingArgsRepeatTheSessionSeek(t *testing.T) {
	for _, startSec := range []float64{1790, 120.5} {
		session := basicHLSParams(testHLSOutDir)
		session.Profile = helpers.HLS_PROFILE_REMUX
		session.CopyVideo = true
		session.StartSec = startSec
		session.VideoStreamIndex = 2
		session.Capabilities = Capabilities{
			Probed:     true,
			CLIOptions: map[string]bool{"readrate": true, "readrate_initial_burst": true},
		}
		sessionArgs := hlsArgs(t, session)

		measureArgs := seekLandingArgs(SeekLandingParams{
			SourcePath:       session.SourcePath,
			VideoStreamIndex: session.VideoStreamIndex,
			StartSec:         startSec,
		})

		sessionInput := sessionArgs[slices.Index(sessionArgs, "-y")+1 : slices.Index(sessionArgs, "-i")]
		readrate := slices.Index(sessionInput, "-readrate")
		if readrate < 0 {
			t.Fatalf("session args lost -readrate, so this test no longer checks it is left out: %v", sessionArgs)
		}
		sessionInput = sessionInput[:readrate]
		measureInput := measureArgs[slices.Index(measureArgs, "-nostdin")+1 : slices.Index(measureArgs, "-i")]
		if !slices.Equal(measureInput, sessionInput) {
			t.Fatalf("measurement input options %v, session input options %v", measureInput, sessionInput)
		}

		wantTail := []string{
			"-i", session.SourcePath,
			"-map", "0:2",
			"-c", "copy",
			"-copyts",
			"-frames:v", "1",
			"-f", "framecrc",
			"pipe:1",
		}
		gotTail := measureArgs[slices.Index(measureArgs, "-i"):]
		if !slices.Equal(gotTail, wantTail) {
			t.Fatalf("measurement output args %v, want %v", gotTail, wantTail)
		}
	}
}

func TestSeekLandingRejectsInvalidParams(t *testing.T) {
	f := &ffmpeg{bin: "/nonexistent/ffmpeg"}
	tests := []struct {
		name   string
		params SeekLandingParams
	}{
		{name: "empty path", params: SeekLandingParams{SourcePath: " ", StartSec: 10}},
		{name: "negative stream", params: SeekLandingParams{SourcePath: "/m.mkv", VideoStreamIndex: -1, StartSec: 10}},
		{name: "zero start", params: SeekLandingParams{SourcePath: "/m.mkv"}},
		{name: "negative start", params: SeekLandingParams{SourcePath: "/m.mkv", StartSec: -3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.SeekLanding(context.Background(), tt.params)
			if err == nil {
				t.Fatal("SeekLanding accepted invalid params")
			}
		})
	}
}

func TestFramecrcFirstPTS(t *testing.T) {
	tests := []struct {
		name    string
		listing string
		want    map[int]float64
	}{
		{
			name: "first packet of each stream in its own time base",
			listing: `#extradata 0:       38, 0xeba80b9c
#software: Lavf62.12.103
#tb 0: 1/1000
#media_type 0: video
#tb 1: 1/48000
#media_type 1: audio
0,    1780153,    1780195,       42,     4775, 0x3eb566da
1,   85449360,   85449360,     1024,      768, 0x00000000
0,    1780195,    1780320,       42,     1200, 0x11111111
1,   85450384,   85450384,     1024,      768, 0x00000000
`,
			want: map[int]float64{0: 1780.195, 1: 85449360.0 / 48000},
		},
		{
			name: "decode time stands in for a missing presentation time",
			listing: `#tb 0: 1/90000
0,     900000, -9223372036854775808,     3003,     4775, 0x3eb566da
`,
			want: map[int]float64{0: 10},
		},
		{
			name: "packets without a time base are skipped",
			listing: `#tb 1: 1/1000
0,       4920,       5000,       40,     4775, 0x3eb566da
`,
			want: map[int]float64{},
		},
		{
			name:    "no packets",
			listing: "#tb 0: 1/1000\n#media_type 0: video\n",
			want:    map[int]float64{},
		},
		{
			name:    "malformed lines",
			listing: "#tb 0: 1/0\n#tb x: 1/1000\nnot,a\n0,a,b,c\n",
			want:    map[int]float64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := framecrcFirstPTS([]byte(tt.listing))
			if len(got) != len(tt.want) {
				t.Fatalf("framecrcFirstPTS = %v, want %v", got, tt.want)
			}
			for stream, want := range tt.want {
				diff := got[stream] - want
				if diff > 1e-9 || diff < -1e-9 {
					t.Fatalf("stream %d first packet at %v s, want %v s", stream, got[stream], want)
				}
			}
		})
	}
}
