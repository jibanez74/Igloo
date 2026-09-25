package keyframeindex_test

import (
	"bytes"
	"context"
	"math"
	"testing"

	"igloo/cmd/internal/keyframeindex"
	"igloo/cmd/internal/keyframeindex/kftestutil"
)

func extractBytes(t *testing.T, data []byte, container string) (keyframeindex.Index, error) {
	t.Helper()
	return keyframeindex.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), container)
}

func requireKeyframes(t *testing.T, idx keyframeindex.Index, want []float64) {
	t.Helper()
	if len(idx.KeyframeSec) != len(want) {
		t.Fatalf("keyframe count = %d, want %d (%v)", len(idx.KeyframeSec), len(want), idx.KeyframeSec)
	}
	for i, kf := range idx.KeyframeSec {
		if math.Abs(kf-want[i]) > 0.001 {
			t.Fatalf("keyframe[%d] = %f, want %f", i, kf, want[i])
		}
	}
}

// mp4Fixture is 10 samples at 512 ticks in a 12800-tick timescale (25 fps,
// 0.04 s per sample), sync samples 1 and 6 -> DTS 0 and 0.2 s.
func mp4Fixture() kftestutil.MP4Options {
	return kftestutil.MP4Options{
		SampleDeltas:       [][2]uint32{{10, 512}},
		SyncSamples:        []uint32{1, 6},
		MediaDurationTicks: 5120,
	}
}
