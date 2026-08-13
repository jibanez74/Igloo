package keyframeindex_test

import (
	"bytes"
	"context"
	"testing"

	"igloo/cmd/internal/keyframeindex"
	"igloo/cmd/internal/keyframeindex/kftestutil"
)

// The fuzzers assert only that hostile bytes never panic or hang; the error
// path is the expected outcome for almost every mutation.

func FuzzExtractEBML(f *testing.F) {
	f.Add(kftestutil.BuildMKV(kftestutil.MKVOptions{CueTimesSec: []float64{0, 4, 8}, DurationSec: 12}))
	f.Add(kftestutil.BuildMKV(kftestutil.MKVOptions{CueTimesSec: []float64{2.5}, OmitSeekHead: true}))
	f.Add(kftestutil.BuildMKV(kftestutil.MKVOptions{CueTimesSec: []float64{0, 7}, ChainSeekHeads: true}))
	f.Add(kftestutil.BuildMKV(kftestutil.MKVOptions{CueTimesSec: []float64{0}, OmitCues: true}))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = keyframeindex.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "mkv")
	})
}

func FuzzExtractISOBMFF(f *testing.F) {
	f.Add(kftestutil.BuildMP4(kftestutil.MP4Options{
		SampleDeltas:       [][2]uint32{{10, 512}},
		SyncSamples:        []uint32{1, 6},
		MediaDurationTicks: 5120,
	}))
	f.Add(kftestutil.BuildMP4(kftestutil.MP4Options{
		SampleDeltas: [][2]uint32{{4, 512}},
		OmitStss:     true,
		MoovAtEnd:    true,
	}))
	f.Add(kftestutil.BuildMP4(kftestutil.MP4Options{
		SampleDeltas: [][2]uint32{{10, 512}},
		SyncSamples:  []uint32{1},
		CttsOffsets:  [][2]int32{{10, 512}},
		Elst:         []kftestutil.ElstEntry{{SegmentDurationMovieTicks: 400, MediaTimeMediaTicks: 512}},
	}))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = keyframeindex.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "mp4")
	})
}
