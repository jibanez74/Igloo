package keyframeindex_test

import (
	"errors"
	"math"
	"testing"

	"igloo/cmd/internal/keyframeindex"
	"igloo/cmd/internal/keyframeindex/kftestutil"
)

// mp4Fixture is 10 samples at 512 ticks in a 12800-tick timescale (25 fps,
// 0.04 s per sample), sync samples 1 and 6 -> DTS 0 and 0.2 s.
func mp4Fixture() kftestutil.MP4Options {
	return kftestutil.MP4Options{
		SampleDeltas:       [][2]uint32{{10, 512}},
		SyncSamples:        []uint32{1, 6},
		MediaDurationTicks: 5120,
	}
}

func TestExtractISOBMFF_SyncSampleTimes(t *testing.T) {
	data := kftestutil.BuildMP4(mp4Fixture())

	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	requireKeyframes(t, idx, []float64{0, 0.2})
	if math.Abs(idx.DurationSec-0.4) > 0.001 {
		t.Fatalf("DurationSec = %f, want 0.4", idx.DurationSec)
	}
}

func TestExtractISOBMFF_MovAndM4vDispatch(t *testing.T) {
	data := kftestutil.BuildMP4(mp4Fixture())

	for _, container := range []string{"mov", "m4v"} {
		idx, err := extractBytes(t, data, container)
		if err != nil {
			t.Fatalf("Extract(%s) returned error: %v", container, err)
		}
		requireKeyframes(t, idx, []float64{0, 0.2})
	}
}

func TestExtractISOBMFF_CttsShiftsToPresentationTime(t *testing.T) {
	opts := mp4Fixture()
	// Every sample presented one sample-duration late.
	opts.CttsOffsets = [][2]int32{{10, 512}}

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0.04, 0.24})
}

func TestExtractISOBMFF_CttsVersion1NegativeOffsets(t *testing.T) {
	opts := mp4Fixture()
	opts.CttsVersion = 1
	opts.CttsOffsets = [][2]int32{{1, 0}, {9, -256}}

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	// Sample 1: DTS 0 + 0. Sample 6: DTS 0.2 - 0.02.
	requireKeyframes(t, idx, []float64{0, 0.18})
}

func TestExtractISOBMFF_ElstMediaEditShiftsTimes(t *testing.T) {
	opts := mp4Fixture()
	opts.CttsOffsets = [][2]int32{{10, 1024}}
	// The classic libx264 shape: one media edit skipping the initial
	// composition delay of 1024 ticks (0.08 s).
	opts.Elst = []kftestutil.ElstEntry{{SegmentDurationMovieTicks: 400, MediaTimeMediaTicks: 1024}}

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	// Raw PTS 0.08 and 0.28 shifted back by 0.08.
	requireKeyframes(t, idx, []float64{0, 0.2})
}

func TestExtractISOBMFF_ElstEmptyPlusMediaEdit(t *testing.T) {
	opts := mp4Fixture()
	// Empty edit of 100 movie ticks (0.1 s at the 1000 default), then a
	// media edit at 512 ticks (0.04 s).
	opts.Elst = []kftestutil.ElstEntry{
		{SegmentDurationMovieTicks: 100, MediaTimeMediaTicks: -1},
		{SegmentDurationMovieTicks: 300, MediaTimeMediaTicks: 512},
	}

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	// Shift = +0.1 - 0.04 = +0.06.
	requireKeyframes(t, idx, []float64{0.06, 0.26})
}

func TestExtractISOBMFF_UnsupportedElstReportsNoIndex(t *testing.T) {
	opts := mp4Fixture()
	opts.Elst = []kftestutil.ElstEntry{
		{SegmentDurationMovieTicks: 100, MediaTimeMediaTicks: 0},
		{SegmentDurationMovieTicks: 100, MediaTimeMediaTicks: 512},
		{SegmentDurationMovieTicks: 100, MediaTimeMediaTicks: 1024},
	}

	data := kftestutil.BuildMP4(opts)
	_, err := extractBytes(t, data, "mp4")
	if !errors.Is(err, keyframeindex.ErrNoIndex) {
		t.Fatalf("error = %v, want ErrNoIndex", err)
	}
}

func TestExtractISOBMFF_MoovAtEnd(t *testing.T) {
	opts := mp4Fixture()
	opts.MoovAtEnd = true

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 0.2})
}

func TestExtractISOBMFF_LargesizeMdatSkipped(t *testing.T) {
	opts := mp4Fixture()
	opts.MoovAtEnd = true
	opts.LargesizeMdat = true

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 0.2})
}

func TestExtractISOBMFF_SkipsNonVideoTrack(t *testing.T) {
	opts := mp4Fixture()
	opts.AudioTrackFirst = true

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 0.2})
}

func TestExtractISOBMFF_AbsentStssMeansAllSamplesSync(t *testing.T) {
	opts := mp4Fixture()
	opts.OmitStss = true
	opts.SampleDeltas = [][2]uint32{{4, 512}}

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	requireKeyframes(t, idx, []float64{0, 0.04, 0.08, 0.12})
}

func TestExtractISOBMFF_DurationClampedToLastKeyframe(t *testing.T) {
	opts := mp4Fixture()
	opts.MediaDurationTicks = 256 // header lies short: 0.02 s

	data := kftestutil.BuildMP4(opts)
	idx, err := extractBytes(t, data, "mp4")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if idx.DurationSec < 0.2 {
		t.Fatalf("DurationSec = %f, want clamp to last keyframe 0.2", idx.DurationSec)
	}
}

func TestExtractISOBMFF_TruncatedFileErrorsCleanly(t *testing.T) {
	data := kftestutil.BuildMP4(mp4Fixture())

	for _, cut := range []int{4, 12, 40, len(data) / 2} {
		_, err := extractBytes(t, data[:cut], "mp4")
		if err == nil {
			t.Fatalf("truncation at %d bytes did not error", cut)
		}
	}
}

func TestExtractISOBMFF_MissingMoovErrors(t *testing.T) {
	data := kftestutil.BuildMP4(mp4Fixture())
	// Keep only ftyp (first box).
	ftypLen := 8 + 24
	_, err := extractBytes(t, data[:ftypLen], "mp4")
	if err == nil {
		t.Fatal("file without moov did not error")
	}
}

func TestExtractISOBMFF_SyncSampleOutsideTrackErrors(t *testing.T) {
	opts := mp4Fixture()
	opts.SyncSamples = []uint32{1, 999}

	data := kftestutil.BuildMP4(opts)
	_, err := extractBytes(t, data, "mp4")
	if err == nil {
		t.Fatal("out-of-range sync sample did not error")
	}
}
