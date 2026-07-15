package ffmpeg

import (
	"testing"

	"igloo/cmd/internal/ffmpeg/fmp4testutil"
)

func FuzzRemuxParsers(f *testing.F) {
	f.Add(fmp4testutil.BuildInitMP4())
	f.Add(fmp4testutil.BuildSegment(fmp4testutil.BuildVideoSample(true), true))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = parseVideoTrackConfig(data)
		_, _ = listDirectChildBoxes(data, 0, len(data))
		_, _ = validateSegmentVideoTrack(data, 1, 4)
	})
}
