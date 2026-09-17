package keyframeindex_test

import (
	"errors"
	"math"
	"testing"

	"igloo/cmd/internal/keyframeindex"
)

// Finalize is the shared gate for both extracted and persisted indexes, so it
// must reject anything a binary search cannot answer against.
func TestFinalize_RejectsUnusableValues(t *testing.T) {
	tests := []struct {
		name string
		idx  keyframeindex.Index
	}{
		{name: "no keyframes", idx: keyframeindex.Index{DurationSec: 10}},
		{name: "NaN keyframe", idx: keyframeindex.Index{KeyframeSec: []float64{0, math.NaN()}, DurationSec: 10}},
		{name: "infinite keyframe", idx: keyframeindex.Index{KeyframeSec: []float64{0, math.Inf(1)}, DurationSec: 10}},
		{name: "negative keyframe", idx: keyframeindex.Index{KeyframeSec: []float64{-1, 4}, DurationSec: 10}},
		{name: "NaN duration", idx: keyframeindex.Index{KeyframeSec: []float64{0, 4}, DurationSec: math.NaN()}},
		{name: "negative duration", idx: keyframeindex.Index{KeyframeSec: []float64{0, 4}, DurationSec: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := keyframeindex.Finalize(tt.idx)
			if !errors.Is(err, keyframeindex.ErrNoIndex) {
				t.Fatalf("Finalize error = %v, want ErrNoIndex", err)
			}
		})
	}
}

func TestFinalize_SortsDedupesAndCoversDuration(t *testing.T) {
	idx, err := keyframeindex.Finalize(keyframeindex.Index{
		KeyframeSec: []float64{9.96, 0, 4.2, 4.2},
		DurationSec: 5,
	})
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	want := []float64{0, 4.2, 9.96}
	if len(idx.KeyframeSec) != len(want) {
		t.Fatalf("KeyframeSec = %v, want %v", idx.KeyframeSec, want)
	}
	for i := range want {
		if idx.KeyframeSec[i] != want[i] {
			t.Fatalf("KeyframeSec = %v, want %v", idx.KeyframeSec, want)
		}
	}
	if idx.DurationSec != 9.96 {
		t.Fatalf("DurationSec = %v, want the last keyframe 9.96", idx.DurationSec)
	}
}
