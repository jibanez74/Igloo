package keyframeindex_test

import (
	"bytes"
	"context"
	"errors"
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

func TestExtractEBML_CuedFile(t *testing.T) {
	cues := []float64{0, 4.2, 9.96, 14.0}
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec: cues,
		DurationSec: 20,
	})

	idx, err := extractBytes(t, data, "mkv")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	requireKeyframes(t, idx, cues)
	if math.Abs(idx.DurationSec-20) > 0.001 {
		t.Fatalf("DurationSec = %f, want 20", idx.DurationSec)
	}
}

func TestExtractEBML_WebmContainerDispatches(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{CueTimesSec: []float64{0, 5}})

	idx, err := extractBytes(t, data, "webm")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 5})
}

func TestExtractEBML_MissingSeekHeadFallsBackToLinearWalk(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec:  []float64{0, 3.5},
		OmitSeekHead: true,
	})

	idx, err := extractBytes(t, data, "mkv")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 3.5})
}

func TestExtractEBML_FollowsChainedSeekHeads(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec:    []float64{0, 7.25},
		ChainSeekHeads: true,
	})

	idx, err := extractBytes(t, data, "mkv")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 7.25})
}

func TestExtractEBML_MissingCuesReportsNoIndex(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec: []float64{0, 5},
		OmitCues:    true,
	})

	_, err := extractBytes(t, data, "mkv")
	if !errors.Is(err, keyframeindex.ErrNoIndex) {
		t.Fatalf("error = %v, want ErrNoIndex", err)
	}
}

func TestExtractEBML_FiltersCuesToVideoTrack(t *testing.T) {
	// Audio cue points share the same times here, so success is measured by
	// the count not doubling.
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec:   []float64{0, 4, 8},
		CueExtraTrack: true,
	})

	idx, err := extractBytes(t, data, "mkv")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 4, 8})
}

func TestExtractEBML_OnlyNonVideoCuesReportsNoIndex(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec:       []float64{0, 4},
		CueOnlyExtraTrack: true,
	})

	_, err := extractBytes(t, data, "mkv")
	if !errors.Is(err, keyframeindex.ErrNoIndex) {
		t.Fatalf("error = %v, want ErrNoIndex", err)
	}
}

func TestExtractEBML_NonDefaultTimestampScale(t *testing.T) {
	// 100 microsecond ticks instead of the 1 ms default.
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec:      []float64{0, 6.283},
		TimestampScaleNs: 100_000,
		DurationSec:      10,
	})

	idx, err := extractBytes(t, data, "mkv")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 6.283})
	if math.Abs(idx.DurationSec-10) > 0.001 {
		t.Fatalf("DurationSec = %f, want 10", idx.DurationSec)
	}
}

func TestExtractEBML_LeadingVoidElement(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec:      []float64{2.5},
		OmitSeekHead:     true,
		LeadingVoidBytes: 64,
	})

	idx, err := extractBytes(t, data, "mkv")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{2.5})
}

func TestExtractEBML_DurationClampedToLastKeyframe(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{
		CueTimesSec: []float64{0, 30},
		DurationSec: 12, // header lies short
	})

	idx, err := extractBytes(t, data, "mkv")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if idx.DurationSec < 30 {
		t.Fatalf("DurationSec = %f, want clamp to last keyframe 30", idx.DurationSec)
	}
}

func TestExtractEBML_TruncatedFileErrorsCleanly(t *testing.T) {
	data := kftestutil.BuildMKV(kftestutil.MKVOptions{CueTimesSec: []float64{0, 5}})

	for _, cut := range []int{1, 5, 20, len(data) / 2, len(data) - 3} {
		_, err := extractBytes(t, data[:cut], "mkv")
		if err == nil {
			t.Fatalf("truncation at %d bytes did not error", cut)
		}
	}
}

func TestExtractEBML_GarbageInputErrors(t *testing.T) {
	garbage := bytes.Repeat([]byte{0xAB, 0x00, 0xFF, 0x13}, 64)
	_, err := extractBytes(t, garbage, "mkv")
	if err == nil {
		t.Fatal("garbage input did not error")
	}
}

func TestExtract_UnsupportedContainer(t *testing.T) {
	_, err := extractBytes(t, []byte("RIFFxxxxAVI LIST"), "avi")
	if !errors.Is(err, keyframeindex.ErrUnsupportedContainer) {
		t.Fatalf("error = %v, want ErrUnsupportedContainer", err)
	}
}

func TestExtract_EmptyFileErrors(t *testing.T) {
	_, err := extractBytes(t, nil, "mkv")
	if err == nil {
		t.Fatal("empty file did not error")
	}
}
