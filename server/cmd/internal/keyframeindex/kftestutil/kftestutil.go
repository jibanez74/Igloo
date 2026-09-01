// Package kftestutil builds tiny, deterministic Matroska and MP4 files for
// keyframeindex tests: real container structure, stub media payloads. The
// byte-level builders let tests exercise structural edge cases (missing
// SeekHead, chained SeekHeads, edit lists, moov placement) that no encoder
// flag can produce on demand.
package kftestutil

import (
	"encoding/binary"
	"math"
)

// MKVOptions shapes a BuildMKV fixture.
type MKVOptions struct {
	// CueTimesSec become CuePoints for the video track, in Matroska ticks
	// derived from TimestampScale.
	CueTimesSec []float64
	// TimestampScaleNs is the Info TimestampScale; zero means the Matroska
	// default of 1_000_000 ns.
	TimestampScaleNs uint64
	// DurationSec is the Info Duration; zero omits the element.
	DurationSec float64
	// OmitSeekHead drops the SeekHead so readers must walk top-level elements.
	OmitSeekHead bool
	// ChainSeekHeads makes the first SeekHead reference only a second one
	// placed after the Cluster, which then references Info/Tracks/Cues.
	ChainSeekHeads bool
	// OmitCues drops the Cues element entirely.
	OmitCues bool
	// CueExtraTrack adds a second (audio) track and cues its track number
	// alongside the video entries, verifying track filtering.
	CueExtraTrack bool
	// CueOnlyExtraTrack cues ONLY the audio track, so video-filtered parsing
	// finds nothing.
	CueOnlyExtraTrack bool
	// LeadingVoidBytes inserts a Void element of this payload size before the
	// first real Segment child.
	LeadingVoidBytes int
}

const (
	mkvVideoTrackNumber = 1
	mkvAudioTrackNumber = 2
	defaultScaleNs      = 1_000_000

	// Fixed timescales for MP4 fixtures: the media (mdhd) timescale every
	// video trak is built with, and the movie (mvhd) timescale edit-list
	// durations are expressed in.
	mp4FixtureTimescale      = 12800
	mp4FixtureMovieTimescale = 1000
)

// BuildMKV renders a minimal but structurally valid Matroska file.
func BuildMKV(opts MKVOptions) []byte {
	scale := opts.TimestampScaleNs
	if scale == 0 {
		scale = defaultScaleNs
	}

	info := buildMKVInfo(scale, opts.DurationSec)
	tracks := buildMKVTracks(opts.CueExtraTrack || opts.CueOnlyExtraTrack)
	cluster := ebmlElement(0x1F43B675, ebmlElement(0xE7, ebmlUintPayload(0))) // Timestamp 0
	var cues []byte
	if !opts.OmitCues {
		cues = buildMKVCues(opts, scale)
	}

	var void []byte
	if opts.LeadingVoidBytes > 0 {
		void = ebmlElement(0xEC, make([]byte, opts.LeadingVoidBytes))
	}

	var segmentPayload []byte
	switch {
	case opts.OmitSeekHead:
		segmentPayload = concat(void, info, tracks, cluster, cues)
	case opts.ChainSeekHeads:
		// First SeekHead points only at the second, which lives after the
		// cluster and references the real elements.
		firstLen := seekHeadLen(1)
		secondStart := firstLen + len(void) + len(info) + len(tracks) + len(cluster)
		infoStart := firstLen + len(void)
		tracksStart := infoStart + len(info)
		cuesStart := secondStart + seekHeadLen(3)
		first := buildSeekHead(seekEntry{0x114D9B74, secondStart})
		second := buildSeekHead(
			seekEntry{0x1549A966, infoStart},
			seekEntry{0x1654AE6B, tracksStart},
			seekEntry{0x1C53BB6B, cuesStart},
		)
		segmentPayload = concat(first, void, info, tracks, cluster, second, cues)
	default:
		headLen := seekHeadLen(3)
		infoStart := headLen + len(void)
		tracksStart := infoStart + len(info)
		cuesStart := tracksStart + len(tracks) + len(cluster)
		head := buildSeekHead(
			seekEntry{0x1549A966, infoStart},
			seekEntry{0x1654AE6B, tracksStart},
			seekEntry{0x1C53BB6B, cuesStart},
		)
		segmentPayload = concat(head, void, info, tracks, cluster, cues)
	}

	ebmlHeader := ebmlElement(0x1A45DFA3, concat(
		ebmlElement(0x4286, ebmlUintPayload(1)), // EBMLVersion
		ebmlElement(0x42F7, ebmlUintPayload(1)), // EBMLReadVersion
		ebmlElement(0x4282, []byte("matroska")), // DocType
		ebmlElement(0x4287, ebmlUintPayload(4)), // DocTypeVersion
		ebmlElement(0x4285, ebmlUintPayload(2)), // DocTypeReadVersion
	))
	segment := ebmlElement(0x18538067, segmentPayload)

	return concat(ebmlHeader, segment)
}

func buildMKVInfo(scaleNs uint64, durationSec float64) []byte {
	children := ebmlElement(0x2AD7B1, ebmlUintPayload(scaleNs))
	if durationSec > 0 {
		durationTicks := durationSec * 1e9 / float64(scaleNs)
		raw := make([]byte, 8)
		binary.BigEndian.PutUint64(raw, math.Float64bits(durationTicks))
		children = concat(children, ebmlElement(0x4489, raw))
	}
	return ebmlElement(0x1549A966, children)
}

func buildMKVTracks(withAudio bool) []byte {
	video := ebmlElement(0xAE, concat(
		ebmlElement(0xD7, ebmlUintPayload(mkvVideoTrackNumber)),
		ebmlElement(0x83, ebmlUintPayload(1)), // video
	))
	if !withAudio {
		return ebmlElement(0x1654AE6B, video)
	}
	audio := ebmlElement(0xAE, concat(
		ebmlElement(0xD7, ebmlUintPayload(mkvAudioTrackNumber)),
		ebmlElement(0x83, ebmlUintPayload(2)), // audio
	))
	// Audio first, so "first video track" selection is exercised.
	return ebmlElement(0x1654AE6B, concat(audio, video))
}

func buildMKVCues(opts MKVOptions, scaleNs uint64) []byte {
	var points []byte
	for _, sec := range opts.CueTimesSec {
		ticks := uint64(math.Round(sec * 1e9 / float64(scaleNs)))
		if !opts.CueOnlyExtraTrack {
			points = concat(points, buildCuePoint(ticks, mkvVideoTrackNumber))
		}
		if opts.CueExtraTrack || opts.CueOnlyExtraTrack {
			points = concat(points, buildCuePoint(ticks, mkvAudioTrackNumber))
		}
	}
	return ebmlElement(0x1C53BB6B, points)
}

func buildCuePoint(timeTicks, track uint64) []byte {
	positions := ebmlElement(0xB7, concat(
		ebmlElement(0xF7, ebmlUintPayload(track)),
		ebmlElement(0xF1, ebmlUintPayload(0)), // CueClusterPosition (unused by parser)
	))
	return ebmlElement(0xBB, concat(
		ebmlElement(0xB3, ebmlUintPayload(timeTicks)),
		positions,
	))
}

type seekEntry struct {
	targetID uint32
	position int
}

// buildSeekHead renders a SeekHead with fixed-width position fields so its
// size is predictable before positions are known (seekHeadLen must match).
func buildSeekHead(entries ...seekEntry) []byte {
	var payload []byte
	for _, entry := range entries {
		idBytes := encodeEBMLID(entry.targetID)
		position := make([]byte, 8)
		binary.BigEndian.PutUint64(position, uint64(entry.position))
		payload = concat(payload, ebmlElement(0x4DBB, concat(
			ebmlElement(0x53AB, idBytes),
			ebmlElement(0x53AC, position),
		)))
	}
	return ebmlElement(0x114D9B74, payload)
}

// seekHeadLen precomputes buildSeekHead's total size for entry IDs of 4 bytes
// (Info/Tracks/Cues/SeekHead all have 4-byte IDs).
func seekHeadLen(entries int) int {
	// Per entry: Seek(2B id + 1B size) wrapping SeekID(2+1+4) + SeekPosition(2+1+8).
	entryLen := 3 + 7 + 11
	payload := entries * entryLen
	// SeekHead: 4-byte ID + size VINT (1 byte while payload < 127).
	return 4 + 1 + payload
}

// --- EBML encoding primitives ---

// ebmlElement wraps a payload for element IDs written with their full marker
// bytes (as Matroska ID constants are). In-payload children encode identically.
func ebmlElement(id uint32, payload []byte) []byte {
	return concat(encodeEBMLID(id), encodeEBMLSize(len(payload)), payload)
}

func encodeEBMLID(id uint32) []byte {
	switch {
	case id > 0xFFFFFF:
		return []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
	case id > 0xFFFF:
		return []byte{byte(id >> 16), byte(id >> 8), byte(id)}
	case id > 0xFF:
		return []byte{byte(id >> 8), byte(id)}
	default:
		return []byte{byte(id)}
	}
}

func encodeEBMLSize(size int) []byte {
	value := uint64(size)
	switch {
	case value < 1<<7-1:
		return []byte{0x80 | byte(value)}
	case value < 1<<14-1:
		return []byte{0x40 | byte(value>>8), byte(value)}
	case value < 1<<21-1:
		return []byte{0x20 | byte(value>>16), byte(value >> 8), byte(value)}
	default:
		return []byte{0x10 | byte(value>>24), byte(value >> 16), byte(value >> 8), byte(value)}
	}
}

func ebmlUintPayload(value uint64) []byte {
	if value == 0 {
		return []byte{0}
	}
	var out []byte
	for shift := 56; shift >= 0; shift -= 8 {
		b := byte(value >> shift)
		if len(out) == 0 && b == 0 {
			continue
		}
		out = append(out, b)
	}
	return out
}

// MP4Options shapes a BuildMP4 fixture. The media and movie timescales are
// fixed at mp4FixtureTimescale and mp4FixtureMovieTimescale.
type MP4Options struct {
	// SampleDeltas is the stts table as (count, delta) pairs.
	SampleDeltas [][2]uint32
	// SyncSamples are 1-based stss entries; nil with OmitStss=false writes an
	// empty stss, nil with OmitStss=true omits the box (all samples sync).
	SyncSamples []uint32
	// OmitStss drops the stss box entirely.
	OmitStss bool
	// CttsOffsets is the ctts table as (count, offset) pairs; nil omits ctts.
	CttsOffsets [][2]int32
	// CttsVersion selects ctts version 0 or 1.
	CttsVersion byte
	// Elst controls the edit list: nil omits edts/elst.
	Elst []ElstEntry
	// MediaDurationTicks sets the mdhd duration in media ticks.
	MediaDurationTicks uint64
	// MoovAtEnd places moov after mdat (the un-faststarted layout).
	MoovAtEnd bool
	// LargesizeMdat writes mdat with a 64-bit largesize header.
	LargesizeMdat bool
	// AudioTrackFirst prepends a 'soun' trak before the video trak.
	AudioTrackFirst bool
}

// ElstEntry is one edit-list entry; MediaTime -1 is an empty edit.
type ElstEntry struct {
	SegmentDurationMovieTicks uint64
	MediaTimeMediaTicks       int64
}

// BuildMP4 renders a minimal but structurally valid MP4 file.
func BuildMP4(opts MP4Options) []byte {
	ftyp := mp4Box("ftyp", concat([]byte("isom"), u32(0x200), []byte("isomiso2avc1mp41")))

	var mdat []byte
	if opts.LargesizeMdat {
		payload := make([]byte, 32)
		header := make([]byte, 16)
		binary.BigEndian.PutUint32(header[0:4], 1)
		copy(header[4:8], "mdat")
		binary.BigEndian.PutUint64(header[8:16], uint64(16+len(payload)))
		mdat = concat(header, payload)
	} else {
		mdat = mp4Box("mdat", make([]byte, 32))
	}

	moov := buildMoov(opts)

	if opts.MoovAtEnd {
		return concat(ftyp, mdat, moov)
	}
	return concat(ftyp, moov, mdat)
}

func buildMoov(opts MP4Options) []byte {
	mvhdPayload := concat(
		[]byte{0, 0, 0, 0}, // version 0 + flags
		u32(0), u32(0),     // creation, modification
		u32(mp4FixtureMovieTimescale),
		u32(0), // duration; parsers fall back to the track's mdhd
	)
	// Pad out the remaining fixed mvhd fields (rate..next_track_ID).
	mvhdPayload = concat(mvhdPayload, make([]byte, 80))
	mvhd := mp4Box("mvhd", mvhdPayload)

	videoTrak := buildTrak(opts, "vide", mp4FixtureTimescale)
	if opts.AudioTrackFirst {
		audioTrak := buildTrak(MP4Options{
			SampleDeltas: [][2]uint32{{1, 1024}},
			OmitStss:     true,
		}, "soun", 48000)
		return mp4Box("moov", concat(mvhd, audioTrak, videoTrak))
	}
	return mp4Box("moov", concat(mvhd, videoTrak))
}

func buildTrak(opts MP4Options, handler string, timescale uint32) []byte {
	tkhd := mp4Box("tkhd", make([]byte, 84))

	var edts []byte
	if opts.Elst != nil {
		var entries []byte
		for _, entry := range opts.Elst {
			entries = concat(entries,
				u32(uint32(entry.SegmentDurationMovieTicks)),
				u32(uint32(int32(entry.MediaTimeMediaTicks))),
				u32(0x00010000), // media_rate 1.0
			)
		}
		elst := mp4Box("elst", concat(
			[]byte{0, 0, 0, 0},
			u32(uint32(len(opts.Elst))),
			entries,
		))
		edts = mp4Box("edts", elst)
	}

	mdhdPayload := concat(
		[]byte{0, 0, 0, 0},
		u32(0), u32(0),
		u32(timescale),
		u32(uint32(opts.MediaDurationTicks)),
		[]byte{0x55, 0xC4}, // language "und"
		make([]byte, 2),
	)
	mdhd := mp4Box("mdhd", mdhdPayload)

	hdlr := mp4Box("hdlr", concat(
		[]byte{0, 0, 0, 0},
		u32(0),
		[]byte(handler),
		make([]byte, 12),
		[]byte{0},
	))

	var sttsEntries []byte
	for _, pair := range opts.SampleDeltas {
		sttsEntries = concat(sttsEntries, u32(pair[0]), u32(pair[1]))
	}
	stts := mp4Box("stts", concat(
		[]byte{0, 0, 0, 0},
		u32(uint32(len(opts.SampleDeltas))),
		sttsEntries,
	))

	stblChildren := stts

	if opts.CttsOffsets != nil {
		var cttsEntries []byte
		for _, pair := range opts.CttsOffsets {
			cttsEntries = concat(cttsEntries, u32(uint32(pair[0])), u32(uint32(pair[1])))
		}
		ctts := mp4Box("ctts", concat(
			[]byte{opts.CttsVersion, 0, 0, 0},
			u32(uint32(len(opts.CttsOffsets))),
			cttsEntries,
		))
		stblChildren = concat(stblChildren, ctts)
	}

	if !opts.OmitStss {
		var stssEntries []byte
		for _, sample := range opts.SyncSamples {
			stssEntries = concat(stssEntries, u32(sample))
		}
		stss := mp4Box("stss", concat(
			[]byte{0, 0, 0, 0},
			u32(uint32(len(opts.SyncSamples))),
			stssEntries,
		))
		stblChildren = concat(stblChildren, stss)
	}

	stbl := mp4Box("stbl", stblChildren)
	minf := mp4Box("minf", stbl)
	mdia := mp4Box("mdia", concat(mdhd, hdlr, minf))
	return mp4Box("trak", concat(tkhd, edts, mdia))
}

func mp4Box(boxType string, payload []byte) []byte {
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], uint32(8+len(payload)))
	copy(header[4:8], boxType)
	return concat(header, payload)
}

func u32(value uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, value)
	return out
}

func concat(parts ...[]byte) []byte {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	out := make([]byte, 0, total)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}
