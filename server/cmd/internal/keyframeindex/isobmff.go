package keyframeindex

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
)

// extractISOBMFF indexes an mp4/m4v/mov file from its sample tables. The moov
// box (front or back of the file) is read in one sequential pass and walked in
// memory; no media data is touched.
func extractISOBMFF(ctx context.Context, r io.ReaderAt, size int64) (Index, error) {
	moov, err := readMoovPayload(ctx, r, size)
	if err != nil {
		return Index{}, err
	}

	movieTimescale, movieDurationSec := parseMvhd(moov)

	trak, found, err := findVideoTrak(moov)
	if err != nil {
		return Index{}, err
	}
	if !found {
		return Index{}, fmt.Errorf("missing video track")
	}

	keyframes, mediaDurationSec, err := parseVideoTrakKeyframes(moov, trak, movieTimescale)
	if err != nil {
		return Index{}, err
	}

	durationSec := mediaDurationSec
	if durationSec <= 0 {
		durationSec = movieDurationSec
	}

	return Index{
		KeyframeSec: keyframes,
		DurationSec: durationSec,
	}, nil
}

// readMoovPayload walks top-level boxes to find moov wherever the muxer put
// it and reads its payload with a single sequential read.
func readMoovPayload(ctx context.Context, r io.ReaderAt, size int64) ([]byte, error) {
	offset := int64(0)
	for offset < size {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}

		header := make([]byte, 16)
		n, err := r.ReadAt(header, offset)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read box header at %d: %w", offset, err)
		}
		if n < 8 {
			return nil, fmt.Errorf("truncated box header at %d", offset)
		}

		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		payloadStart := offset + 8
		switch boxSize {
		case 0:
			// Box extends to end of file.
			boxSize = size - offset
		case 1:
			if n < 16 {
				return nil, fmt.Errorf("truncated largesize header at %d", offset)
			}
			largeSize := binary.BigEndian.Uint64(header[8:16])
			if largeSize > uint64(size) {
				return nil, fmt.Errorf("box %q largesize overruns file", boxType)
			}
			boxSize = int64(largeSize)
			payloadStart = offset + 16
		}
		if boxSize < payloadStart-offset || offset+boxSize > size {
			return nil, fmt.Errorf("box %q overruns file", boxType)
		}

		if boxType == "moov" {
			payloadSize := offset + boxSize - payloadStart
			if payloadSize > maxMoovPayloadBytes {
				return nil, ErrNoIndex
			}
			payload := make([]byte, payloadSize)
			_, err = io.ReadFull(io.NewSectionReader(r, payloadStart, payloadSize), payload)
			if err != nil {
				return nil, fmt.Errorf("read moov payload: %w", err)
			}
			return payload, nil
		}

		offset += boxSize
	}

	return nil, fmt.Errorf("missing moov box")
}

// isoBox delimits one box inside an in-memory payload.
type isoBox struct {
	Type         string
	PayloadStart int
	End          int
}

func listISOChildBoxes(data []byte, start, end int) ([]isoBox, error) {
	boxes := make([]isoBox, 0, 8)
	offset := start
	for offset < end {
		if end-offset < 8 {
			return nil, fmt.Errorf("truncated box header")
		}

		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		boxType := string(data[offset+4 : offset+8])
		payloadStart := offset + 8
		if size == 1 {
			if end-offset < 16 {
				return nil, fmt.Errorf("truncated largesize header")
			}
			largeSize := binary.BigEndian.Uint64(data[offset+8 : offset+16])
			if largeSize > uint64(end-offset) {
				return nil, fmt.Errorf("box %q overruns its parent", boxType)
			}
			size = int(largeSize)
			payloadStart = offset + 16
		}
		if size == 0 {
			size = end - offset
		}
		if size < payloadStart-offset || offset+size > end {
			return nil, fmt.Errorf("box %q overruns its parent", boxType)
		}

		boxes = append(boxes, isoBox{
			Type:         boxType,
			PayloadStart: payloadStart,
			End:          offset + size,
		})
		offset += size
	}
	return boxes, nil
}

func findISOChildBox(data []byte, start, end int, boxType string) (isoBox, bool, error) {
	boxes, err := listISOChildBoxes(data, start, end)
	if err != nil {
		return isoBox{}, false, err
	}
	for _, box := range boxes {
		if box.Type == boxType {
			return box, true, nil
		}
	}
	return isoBox{}, false, nil
}

// parseMvhd returns the movie timescale and duration in seconds; zeros when
// the box is absent or malformed (callers treat them as fallbacks only).
func parseMvhd(moov []byte) (uint64, float64) {
	mvhd, found, err := findISOChildBox(moov, 0, len(moov), "mvhd")
	if err != nil || !found {
		return 0, 0
	}

	payload := moov[mvhd.PayloadStart:mvhd.End]
	if len(payload) < 4 {
		return 0, 0
	}

	var timescale uint64
	var duration uint64
	if payload[0] == 1 {
		if len(payload) < 4+8+8+4+8 {
			return 0, 0
		}
		timescale = uint64(binary.BigEndian.Uint32(payload[20:24]))
		duration = binary.BigEndian.Uint64(payload[24:32])
	} else {
		if len(payload) < 4+4+4+4+4 {
			return 0, 0
		}
		timescale = uint64(binary.BigEndian.Uint32(payload[12:16]))
		duration = uint64(binary.BigEndian.Uint32(payload[16:20]))
	}
	if timescale == 0 {
		return 0, 0
	}
	return timescale, float64(duration) / float64(timescale)
}

func findVideoTrak(moov []byte) (isoBox, bool, error) {
	boxes, err := listISOChildBoxes(moov, 0, len(moov))
	if err != nil {
		return isoBox{}, false, err
	}

	for _, trak := range boxes {
		if trak.Type != "trak" {
			continue
		}
		mdia, found, findErr := findISOChildBox(moov, trak.PayloadStart, trak.End, "mdia")
		if findErr != nil {
			return isoBox{}, false, findErr
		}
		if !found {
			continue
		}
		hdlr, found, findErr := findISOChildBox(moov, mdia.PayloadStart, mdia.End, "hdlr")
		if findErr != nil {
			return isoBox{}, false, findErr
		}
		if !found {
			continue
		}
		payload := moov[hdlr.PayloadStart:hdlr.End]
		if len(payload) >= 12 && string(payload[8:12]) == "vide" {
			return trak, true, nil
		}
	}

	return isoBox{}, false, nil
}

func parseVideoTrakKeyframes(moov []byte, trak isoBox, movieTimescale uint64) ([]float64, float64, error) {
	mdia, found, err := findISOChildBox(moov, trak.PayloadStart, trak.End, "mdia")
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return nil, 0, fmt.Errorf("missing mdia box")
	}

	mediaTimescale, mediaDurationSec, err := parseMdhd(moov, mdia)
	if err != nil {
		return nil, 0, err
	}

	minf, found, err := findISOChildBox(moov, mdia.PayloadStart, mdia.End, "minf")
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return nil, 0, fmt.Errorf("missing minf box")
	}
	stbl, found, err := findISOChildBox(moov, minf.PayloadStart, minf.End, "stbl")
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return nil, 0, fmt.Errorf("missing stbl box")
	}

	deltas, totalSamples, err := parseStts(moov, stbl)
	if err != nil {
		return nil, 0, err
	}
	offsets, err := parseCtts(moov, stbl)
	if err != nil {
		return nil, 0, err
	}
	syncSamples, err := parseStss(moov, stbl, totalSamples)
	if err != nil {
		return nil, 0, err
	}

	ptsShiftSec, err := parseElstShift(moov, trak, mediaTimescale, movieTimescale)
	if err != nil {
		return nil, 0, err
	}

	keyframes := make([]float64, 0, len(syncSamples))
	for _, sample := range syncSamples {
		dts, ok := deltas.accumulatedAt(sample)
		if !ok {
			return nil, 0, fmt.Errorf("sync sample %d outside stts table", sample)
		}
		cts := dts
		if offsets != nil {
			offset, offsetOK := offsets.valueAt(sample)
			if offsetOK {
				cts += offset
			}
		}
		pts := float64(cts)/float64(mediaTimescale) + ptsShiftSec
		if pts < 0 {
			pts = 0
		}
		keyframes = append(keyframes, pts)
	}

	return keyframes, mediaDurationSec, nil
}

func parseMdhd(moov []byte, mdia isoBox) (uint64, float64, error) {
	mdhd, found, err := findISOChildBox(moov, mdia.PayloadStart, mdia.End, "mdhd")
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, fmt.Errorf("missing mdhd box")
	}

	payload := moov[mdhd.PayloadStart:mdhd.End]
	if len(payload) < 4 {
		return 0, 0, fmt.Errorf("invalid mdhd payload")
	}

	var timescale uint64
	var duration uint64
	if payload[0] == 1 {
		if len(payload) < 4+8+8+4+8 {
			return 0, 0, fmt.Errorf("invalid mdhd payload")
		}
		timescale = uint64(binary.BigEndian.Uint32(payload[20:24]))
		duration = binary.BigEndian.Uint64(payload[24:32])
	} else {
		if len(payload) < 4+4+4+4+4 {
			return 0, 0, fmt.Errorf("invalid mdhd payload")
		}
		timescale = uint64(binary.BigEndian.Uint32(payload[12:16]))
		duration = uint64(binary.BigEndian.Uint32(payload[16:20]))
	}
	if timescale == 0 {
		return 0, 0, fmt.Errorf("mdhd timescale is zero")
	}

	return timescale, float64(duration) / float64(timescale), nil
}

func parseStts(moov []byte, stbl isoBox) (*runLengthTable, uint64, error) {
	stts, found, err := findISOChildBox(moov, stbl.PayloadStart, stbl.End, "stts")
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return nil, 0, fmt.Errorf("missing stts box")
	}

	payload := moov[stts.PayloadStart:stts.End]
	entries, err := parseRunLengthEntries(payload, "stts", func(raw uint32) int64 {
		return int64(raw)
	})
	if err != nil {
		return nil, 0, err
	}

	totalSamples := uint64(0)
	for _, entry := range entries {
		totalSamples += entry.count
	}
	if totalSamples == 0 || totalSamples > maxSampleCount {
		return nil, 0, ErrNoIndex
	}

	return &runLengthTable{entries: entries}, totalSamples, nil
}

// parseCtts returns nil when the box is absent (no composition offsets).
func parseCtts(moov []byte, stbl isoBox) (*runLengthTable, error) {
	ctts, found, err := findISOChildBox(moov, stbl.PayloadStart, stbl.End, "ctts")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	payload := moov[ctts.PayloadStart:ctts.End]
	if len(payload) < 4 {
		return nil, fmt.Errorf("invalid ctts payload")
	}
	version := payload[0]
	entries, err := parseRunLengthEntries(payload, "ctts", func(raw uint32) int64 {
		// Version 1 offsets are signed; version 0 are unsigned but small in
		// practice.
		if version == 1 {
			return int64(int32(raw))
		}
		return int64(raw)
	})
	if err != nil {
		return nil, err
	}

	return &runLengthTable{entries: entries}, nil
}

// parseRunLengthEntries decodes the shared stts/ctts layout: full-box header,
// entry count, then (count, value) uint32 pairs.
func parseRunLengthEntries(payload []byte, boxName string, decode func(raw uint32) int64) ([]runLengthEntry, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("invalid %s payload", boxName)
	}
	entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
	if entryCount < 0 || len(payload) < 8+entryCount*8 {
		return nil, fmt.Errorf("truncated %s table", boxName)
	}

	entries := make([]runLengthEntry, 0, entryCount)
	for i := 0; i < entryCount; i++ {
		base := 8 + i*8
		entries = append(entries, runLengthEntry{
			count: uint64(binary.BigEndian.Uint32(payload[base : base+4])),
			value: decode(binary.BigEndian.Uint32(payload[base+4 : base+8])),
		})
	}
	return entries, nil
}

// parseStss returns ascending 1-based sync sample numbers; an absent stss
// means every sample is sync (capped — an intra-only source gains nothing
// over the probing fallback).
func parseStss(moov []byte, stbl isoBox, totalSamples uint64) ([]uint64, error) {
	stss, found, err := findISOChildBox(moov, stbl.PayloadStart, stbl.End, "stss")
	if err != nil {
		return nil, err
	}
	if !found {
		if totalSamples > maxKeyframeCount {
			return nil, ErrNoIndex
		}
		all := make([]uint64, totalSamples)
		for i := range all {
			all[i] = uint64(i + 1)
		}
		return all, nil
	}

	payload := moov[stss.PayloadStart:stss.End]
	if len(payload) < 8 {
		return nil, fmt.Errorf("invalid stss payload")
	}
	entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
	if entryCount < 0 || len(payload) < 8+entryCount*4 {
		return nil, fmt.Errorf("truncated stss table")
	}
	if entryCount > maxKeyframeCount {
		return nil, ErrNoIndex
	}

	samples := make([]uint64, 0, entryCount)
	for i := 0; i < entryCount; i++ {
		base := 8 + i*4
		sample := uint64(binary.BigEndian.Uint32(payload[base : base+4]))
		if sample == 0 || sample > totalSamples {
			return nil, fmt.Errorf("stss sample %d outside track", sample)
		}
		samples = append(samples, sample)
	}

	// The spec requires ascending order; tolerate violations by sorting so
	// the forward-only table iteration stays valid.
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples, nil
}

// parseElstShift converts the edit list into a constant PTS shift in seconds.
// Supported shapes: no edit list; a single media edit (the common libx264
// composition-delay compensation); a leading empty edit followed by one media
// edit. Anything else aborts extraction — ffprobe's pts_time honors edit
// lists, so serving unshifted times would disagree with the ground truth.
func parseElstShift(moov []byte, trak isoBox, mediaTimescale, movieTimescale uint64) (float64, error) {
	edts, found, err := findISOChildBox(moov, trak.PayloadStart, trak.End, "edts")
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	elst, found, err := findISOChildBox(moov, edts.PayloadStart, edts.End, "elst")
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}

	payload := moov[elst.PayloadStart:elst.End]
	if len(payload) < 8 {
		return 0, fmt.Errorf("invalid elst payload")
	}
	version := payload[0]
	entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
	entrySize := 12
	if version == 1 {
		entrySize = 20
	}
	if entryCount < 0 || len(payload) < 8+entryCount*entrySize {
		return 0, fmt.Errorf("truncated elst table")
	}

	type editEntry struct {
		segmentDuration uint64
		mediaTime       int64
	}
	entries := make([]editEntry, 0, entryCount)
	for i := 0; i < entryCount; i++ {
		base := 8 + i*entrySize
		if version == 1 {
			entries = append(entries, editEntry{
				segmentDuration: binary.BigEndian.Uint64(payload[base : base+8]),
				mediaTime:       int64(binary.BigEndian.Uint64(payload[base+8 : base+16])),
			})
		} else {
			entries = append(entries, editEntry{
				segmentDuration: uint64(binary.BigEndian.Uint32(payload[base : base+4])),
				mediaTime:       int64(int32(binary.BigEndian.Uint32(payload[base+4 : base+8]))),
			})
		}
	}

	shift := 0.0
	switch {
	case len(entries) == 0:
		return 0, nil
	case len(entries) == 1 && entries[0].mediaTime >= 0:
		shift = -float64(entries[0].mediaTime) / float64(mediaTimescale)
	case len(entries) == 2 && entries[0].mediaTime == -1 && entries[1].mediaTime >= 0:
		if movieTimescale == 0 {
			return 0, ErrNoIndex
		}
		shift = float64(entries[0].segmentDuration)/float64(movieTimescale) -
			float64(entries[1].mediaTime)/float64(mediaTimescale)
	default:
		return 0, ErrNoIndex
	}

	return shift, nil
}

// runLengthTable answers per-sample lookups over an ISOBMFF (count, value)
// run-length table with a single forward pass; sample numbers are 1-based and
// must be requested in ascending order.
type runLengthTable struct {
	entries []runLengthEntry
	// entryIdx is the current run; baseSample counts the samples before it,
	// and baseAccum is the value sum across them (used only for stts).
	entryIdx   int
	baseSample uint64
	baseAccum  int64
}

type runLengthEntry struct {
	count uint64
	value int64
}

func (t *runLengthTable) advanceTo(sample uint64) bool {
	for t.entryIdx < len(t.entries) && t.baseSample+t.entries[t.entryIdx].count < sample {
		entry := t.entries[t.entryIdx]
		t.baseAccum += int64(entry.count) * entry.value
		t.baseSample += entry.count
		t.entryIdx++
	}
	return t.entryIdx < len(t.entries)
}

// accumulatedAt returns the sum of values for all samples before the given
// one — for stts, the sample's decode timestamp in media ticks.
func (t *runLengthTable) accumulatedAt(sample uint64) (int64, bool) {
	if sample == 0 || !t.advanceTo(sample) {
		return 0, false
	}
	entry := t.entries[t.entryIdx]
	return t.baseAccum + int64(sample-1-t.baseSample)*entry.value, true
}

// valueAt returns the run value covering the given sample — for ctts, its
// composition offset in media ticks.
func (t *runLengthTable) valueAt(sample uint64) (int64, bool) {
	if sample == 0 || !t.advanceTo(sample) {
		return 0, false
	}
	return t.entries[t.entryIdx].value, true
}
