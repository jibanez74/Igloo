package ffmpeg

import (
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestHLSSegmentCount(t *testing.T) {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)

	tests := []struct {
		name        string
		durationSec float64
		frameRate   float64
		want        int
	}{
		{name: "zero duration still yields one segment", durationSec: 0, frameRate: 24, want: 1},
		{name: "shorter than one frame yields one segment", durationSec: 0.01, frameRate: 24, want: 1},
		{name: "one full segment", durationSec: segDur, frameRate: 24, want: 1},
		{name: "one frame into the second segment", durationSec: segDur + 1.0/24.0, frameRate: 24, want: 2},
		{name: "two full segments", durationSec: segDur * 2, frameRate: 24, want: 2},
		{name: "partial last segment", durationSec: segDur*2 + 1, frameRate: 24, want: 3},
		// The container outlasts its 960th frame by five milliseconds of
		// audio; FFmpeg never sees a frame at 40 s and writes ten segments.
		{name: "sub-frame tail does not add a segment", durationSec: segDur*10 + 0.005, frameRate: 24, want: 10},
		{name: "frame exactly on the boundary adds a segment", durationSec: 961.0 / 24.0, frameRate: 24, want: 11},
		{name: "23.976 fps boundary frame", durationSec: 961.0 / (24000.0 / 1001.0), frameRate: 24000.0 / 1001.0, want: 11},
		{name: "60 fps sub-frame tail", durationSec: segDur*10 + 0.01, frameRate: 60, want: 10},
		{name: "unknown frame rate assumes 24 fps", durationSec: segDur*10 + 0.005, frameRate: 0, want: 10},
		{name: "unknown frame rate keeps a real boundary frame", durationSec: segDur*10 + 0.05, frameRate: 0, want: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HLSSegmentCount(tt.durationSec, tt.frameRate)
			if got != tt.want {
				t.Fatalf("HLSSegmentCount(%v, %v) = %d, want %d", tt.durationSec, tt.frameRate, got, tt.want)
			}
		})
	}
}
