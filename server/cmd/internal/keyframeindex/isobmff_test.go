package keyframeindex_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"strings"
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
	_, err := extractBytes(t, data[:kftestutil.MP4FtypLen()], "mp4")
	if err == nil || !strings.Contains(err.Error(), "missing moov box") {
		t.Fatalf("error = %v, want missing moov box", err)
	}
}

// zeroPaddedReaderAt serves data and reads zeros past it, so a fixture can
// declare a huge box without the test allocating it.
type zeroPaddedReaderAt struct {
	data []byte
}

func (r zeroPaddedReaderAt) ReadAt(p []byte, off int64) (int, error) {
	for i := range p {
		p[i] = 0
	}
	if off < int64(len(r.data)) {
		copy(p, r.data[off:])
	}
	return len(p), nil
}

func TestExtractISOBMFF_CapsRejectOversizedTables(t *testing.T) {
	tests := []struct {
		name string
		opts kftestutil.MP4Options
	}{
		{
			name: "sample count",
			opts: kftestutil.MP4Options{SampleDeltas: [][2]uint32{{4_000_001, 512}}, OmitStss: true},
		},
		{
			name: "keyframe count without stss",
			opts: kftestutil.MP4Options{SampleDeltas: [][2]uint32{{200_001, 512}}, OmitStss: true},
		},
		{
			name: "keyframe count in stss",
			opts: kftestutil.MP4Options{SampleDeltas: [][2]uint32{{200_001, 512}}, SyncSamples: make([]uint32, 200_001)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractBytes(t, kftestutil.BuildMP4(tt.opts), "mp4")
			if !errors.Is(err, keyframeindex.ErrNoIndex) {
				t.Fatalf("error = %v, want ErrNoIndex", err)
			}
		})
	}
}

func TestExtractISOBMFF_OversizedMoovReportsNoIndex(t *testing.T) {
	data := kftestutil.BuildMP4(mp4Fixture())
	moovStart := kftestutil.MP4FtypLen()
	if string(data[moovStart+4:moovStart+8]) != "moov" {
		t.Fatalf("fixture box after ftyp is %q, want moov", data[moovStart+4:moovStart+8])
	}
	// Declare a 33 MiB moov inside a 40 MiB file: the box fits the file, so
	// only the payload cap can reject it, and it must do so before reading.
	binary.BigEndian.PutUint32(data[moovStart:moovStart+4], 33<<20)
	fileSize := int64(40 << 20)

	_, err := keyframeindex.Extract(context.Background(), zeroPaddedReaderAt{data: data}, fileSize, "mp4")
	if !errors.Is(err, keyframeindex.ErrNoIndex) {
		t.Fatalf("error = %v, want ErrNoIndex", err)
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
