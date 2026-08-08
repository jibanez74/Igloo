// Package keyframeindex extracts video keyframe timestamps from a media
// container's own seek tables — Matroska Cues for mkv/webm, sample tables
// (stts/ctts/stss) for mp4/m4v/mov — using a handful of bounded reads instead
// of demuxing the file. FFmpeg's -ss input seeking consults these same
// structures, so the extracted timestamps match where FFmpeg actually lands
// when a copy-video HLS session seeks.
//
// Extraction always indexes the first video track in the container. Files
// with multiple video tracks are rare and the primary track is what playback
// maps, so the distinction is accepted rather than modeled.
package keyframeindex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
)

// Index is one video stream's seek index.
type Index struct {
	// KeyframeSec holds keyframe presentation times in seconds, sorted
	// ascending and deduplicated.
	KeyframeSec []float64
	// DurationSec is the container-declared duration, raised to the last
	// keyframe when the header under-reports (real files do this).
	DurationSec float64
}

// ErrNoIndex means the file was parseable but carries no usable seek index
// (missing Cues, unsupported edit list, oversized tables, ...). Callers fall
// back to probing.
var ErrNoIndex = errors.New("keyframeindex: no seek index available")

// ErrUnsupportedContainer means no parser exists for the container (avi).
var ErrUnsupportedContainer = errors.New("keyframeindex: unsupported container")

// Byte and cardinality caps. A corrupt or hostile file must cost a bounded
// read and a clean fallback, never an OOM or an unbounded network read.
const (
	maxCuesPayloadBytes = 16 << 20
	maxMoovPayloadBytes = 32 << 20
	maxSampleCount      = 4_000_000
	// Beyond this many keyframes the source is effectively intra-only;
	// storing megabyte rows buys nothing over the probing fallback.
	maxKeyframeCount = 200_000
)

// Extract reads the seek index for the first video track. The container name
// is the scanner-stored value (lowercase extension without dot).
func Extract(ctx context.Context, r io.ReaderAt, size int64, container string) (Index, error) {
	if size <= 0 {
		return Index{}, fmt.Errorf("keyframeindex: invalid file size %d", size)
	}

	var idx Index
	var err error
	switch container {
	case "mkv", "webm":
		idx, err = extractEBML(ctx, r, size)
	case "mp4", "m4v", "mov":
		idx, err = extractISOBMFF(ctx, r, size)
	default:
		return Index{}, ErrUnsupportedContainer
	}
	if err != nil {
		return Index{}, err
	}

	return Finalize(idx)
}

// Finalize enforces the shared invariants: sorted, deduplicated, non-empty,
// capped, finite, non-negative, and a duration that covers the last keyframe.
// Extract runs every parsed index through it, and the persistence layer runs
// every index read back out of the database through it, so a corrupt row is
// rejected rather than silently mis-seeking a copy-video session.
func Finalize(idx Index) (Index, error) {
	if len(idx.KeyframeSec) == 0 {
		return Index{}, ErrNoIndex
	}
	if len(idx.KeyframeSec) > maxKeyframeCount {
		return Index{}, ErrNoIndex
	}

	// NaN would survive sorting as an unordered element and break the binary
	// search every seek depends on; a negative timestamp is not a real
	// presentation time.
	for _, kf := range idx.KeyframeSec {
		if math.IsNaN(kf) || math.IsInf(kf, 0) || kf < 0 {
			return Index{}, ErrNoIndex
		}
	}
	if math.IsNaN(idx.DurationSec) || math.IsInf(idx.DurationSec, 0) || idx.DurationSec < 0 {
		return Index{}, ErrNoIndex
	}

	sort.Float64s(idx.KeyframeSec)
	deduped := idx.KeyframeSec[:1]
	for _, kf := range idx.KeyframeSec[1:] {
		if kf != deduped[len(deduped)-1] {
			deduped = append(deduped, kf)
		}
	}
	idx.KeyframeSec = deduped

	if last := idx.KeyframeSec[len(idx.KeyframeSec)-1]; idx.DurationSec < last {
		idx.DurationSec = last
	}

	return idx, nil
}

// checkContext is the per-element cancellation point shared by both parsers.
// Individual reads are size-capped, so checking between reads bounds total
// work without plumbing deadlines into every read call.
func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
