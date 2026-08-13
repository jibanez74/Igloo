package keyframeindex

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Matroska/WebM element IDs (EBML IDs keep their marker bits, so these are
// the literal on-disk byte sequences).
const (
	ebmlHeaderID     = 0x1A45DFA3
	segmentID        = 0x18538067
	seekHeadID       = 0x114D9B74
	seekID           = 0x4DBB
	seekEntryIDID    = 0x53AB
	seekPositionID   = 0x53AC
	infoID           = 0x1549A966
	timestampScaleID = 0x2AD7B1
	durationID       = 0x4489
	tracksID         = 0x1654AE6B
	trackEntryID     = 0xAE
	trackNumberID    = 0xD7
	trackTypeID      = 0x83
	cuesID           = 0x1C53BB6B
	cuePointID       = 0xBB
	cueTimeID        = 0xB3
	cueTrackPosID    = 0xB7
	cueTrackID       = 0xF7
)

const (
	matroskaVideoTrackType = 1
	defaultTimestampScale  = 1_000_000 // nanoseconds per Matroska tick

	// Info, Tracks, and SeekHead are header-sized elements; Cues has its own
	// larger cap. A payload beyond this is corrupt or hostile.
	maxEBMLHeaderPayloadBytes = 4 << 20
	// Top-level Segment children before giving up the linear walk. Real files
	// have a handful of header elements plus one Cluster per few seconds;
	// skipping a Cluster is a seek, not a read.
	maxEBMLTopLevelElements = 10_000
	// SeekHead may reference another SeekHead (commonly one at the front
	// pointing at one next to the Cues at the end). Chains beyond a few hops
	// only occur in corrupt files.
	maxSeekHeadHops = 4

	// ebmlUnknownSize marks an element whose size VINT was all ones.
	ebmlUnknownSize = int64(-1)
)

type ebmlElement struct {
	ID uint32
	// DataStart/DataSize delimit the payload. DataSize is ebmlUnknownSize for
	// unknown-size elements (tolerated only on Segment).
	DataStart int64
	DataSize  int64
}

func extractEBML(ctx context.Context, r io.ReaderAt, size int64) (Index, error) {
	header, err := readEBMLElementHeader(r, 0, size)
	if err != nil {
		return Index{}, fmt.Errorf("read EBML header: %w", err)
	}
	if header.ID != ebmlHeaderID || header.DataSize == ebmlUnknownSize {
		return Index{}, fmt.Errorf("not an EBML file")
	}

	segment, err := readEBMLElementHeader(r, header.DataStart+header.DataSize, size)
	if err != nil {
		return Index{}, fmt.Errorf("read segment header: %w", err)
	}
	if segment.ID != segmentID {
		return Index{}, fmt.Errorf("missing segment element")
	}
	segmentEnd := size
	if segment.DataSize != ebmlUnknownSize {
		segmentEnd = segment.DataStart + segment.DataSize
		if segmentEnd > size {
			segmentEnd = size
		}
	}

	locations, err := locateEBMLIndexElements(ctx, r, segment.DataStart, segmentEnd)
	if err != nil {
		return Index{}, err
	}
	if locations.cues == 0 {
		return Index{}, ErrNoIndex
	}

	timestampScale := float64(defaultTimestampScale)
	durationTicks := 0.0
	if locations.info != 0 {
		scale, duration, infoErr := parseEBMLInfo(r, locations.info, segmentEnd)
		if infoErr != nil {
			return Index{}, infoErr
		}
		if scale > 0 {
			timestampScale = float64(scale)
		}
		durationTicks = duration
	}

	// The video track number filters cue points. When Tracks is unreachable,
	// accept every cue point: files virtually always cue the video track.
	videoTrack := uint64(0)
	if locations.tracks != 0 {
		track, trackErr := parseEBMLVideoTrackNumber(r, locations.tracks, segmentEnd)
		if trackErr != nil {
			return Index{}, trackErr
		}
		videoTrack = track
	}

	if err = checkContext(ctx); err != nil {
		return Index{}, err
	}

	keyframes, err := parseEBMLCues(r, locations.cues, segmentEnd, videoTrack, timestampScale)
	if err != nil {
		return Index{}, err
	}

	return Index{
		KeyframeSec: keyframes,
		DurationSec: durationTicks * timestampScale / 1e9,
	}, nil
}

type ebmlIndexLocations struct {
	// Absolute file offsets of each element's header; zero means not found
	// (offset 0 is always the EBML header, never a Segment child).
	info   int64
	tracks int64
	cues   int64
}

// locateEBMLIndexElements finds Info, Tracks, and Cues, preferring the
// SeekHead so a cued file costs a few seeks. A missing or incomplete SeekHead
// falls back to a bounded linear walk of top-level Segment children, which
// still never reads Cluster payloads — skipping an element is a seek.
func locateEBMLIndexElements(ctx context.Context, r io.ReaderAt, segmentDataStart, segmentEnd int64) (ebmlIndexLocations, error) {
	locations, err := locateViaSeekHead(ctx, r, segmentDataStart, segmentEnd)
	if err != nil {
		return ebmlIndexLocations{}, err
	}
	if locations.cues != 0 {
		return locations, nil
	}

	return locateViaLinearWalk(ctx, r, segmentDataStart, segmentEnd, locations)
}

func locateViaSeekHead(ctx context.Context, r io.ReaderAt, segmentDataStart, segmentEnd int64) (ebmlIndexLocations, error) {
	locations := ebmlIndexLocations{}

	first, err := readEBMLElementHeader(r, segmentDataStart, segmentEnd)
	if err != nil {
		return locations, fmt.Errorf("read first segment child: %w", err)
	}
	if first.ID != seekHeadID {
		return locations, nil
	}

	seekHeadOffset := segmentDataStart
	for hop := 0; hop < maxSeekHeadHops && seekHeadOffset != 0; hop++ {
		if err = checkContext(ctx); err != nil {
			return locations, err
		}

		head, headErr := readEBMLElementHeader(r, seekHeadOffset, segmentEnd)
		if headErr != nil {
			return locations, fmt.Errorf("read seekhead: %w", headErr)
		}
		if head.ID != seekHeadID || head.DataSize == ebmlUnknownSize || head.DataSize > maxEBMLHeaderPayloadBytes {
			return locations, nil
		}

		payload := make([]byte, head.DataSize)
		_, readErr := r.ReadAt(payload, head.DataStart)
		if readErr != nil {
			return locations, fmt.Errorf("read seekhead payload: %w", readErr)
		}

		nextSeekHead := int64(0)
		walkErr := walkEBMLChildren(payload, func(id uint32, data []byte) error {
			if id != seekID {
				return nil
			}
			targetID, position, entryErr := parseEBMLSeekEntry(data)
			if entryErr != nil {
				return entryErr
			}
			// SeekPosition is relative to the start of the Segment payload.
			absolute := segmentDataStart + int64(position)
			if absolute <= 0 || absolute >= segmentEnd {
				return nil
			}
			switch targetID {
			case infoID:
				locations.info = absolute
			case tracksID:
				locations.tracks = absolute
			case cuesID:
				locations.cues = absolute
			case seekHeadID:
				nextSeekHead = absolute
			}
			return nil
		})
		if walkErr != nil {
			return locations, walkErr
		}

		if locations.cues != 0 {
			return locations, nil
		}
		seekHeadOffset = nextSeekHead
	}

	return locations, nil
}

func locateViaLinearWalk(
	ctx context.Context,
	r io.ReaderAt,
	segmentDataStart, segmentEnd int64,
	locations ebmlIndexLocations,
) (ebmlIndexLocations, error) {
	offset := segmentDataStart
	for element := 0; element < maxEBMLTopLevelElements && offset < segmentEnd; element++ {
		if err := checkContext(ctx); err != nil {
			return locations, err
		}

		child, err := readEBMLElementHeader(r, offset, segmentEnd)
		if err != nil {
			return locations, fmt.Errorf("walk segment children: %w", err)
		}
		if child.DataSize == ebmlUnknownSize {
			// Only Segment may be unknown-size; an unknown-size Cluster makes
			// skipping impossible.
			return locations, ErrNoIndex
		}

		switch child.ID {
		case infoID:
			if locations.info == 0 {
				locations.info = offset
			}
		case tracksID:
			if locations.tracks == 0 {
				locations.tracks = offset
			}
		case cuesID:
			locations.cues = offset
			return locations, nil
		}

		offset = child.DataStart + child.DataSize
	}

	return locations, nil
}

func parseEBMLInfo(r io.ReaderAt, offset, segmentEnd int64) (uint64, float64, error) {
	payload, err := readEBMLBoundedPayload(r, offset, segmentEnd, infoID, maxEBMLHeaderPayloadBytes)
	if err != nil {
		return 0, 0, err
	}

	scale := uint64(0)
	duration := 0.0
	walkErr := walkEBMLChildren(payload, func(id uint32, data []byte) error {
		switch id {
		case timestampScaleID:
			scale = ebmlUint(data)
		case durationID:
			parsed, ok := ebmlFloat(data)
			if ok {
				duration = parsed
			}
		}
		return nil
	})
	if walkErr != nil {
		return 0, 0, walkErr
	}

	return scale, duration, nil
}

func parseEBMLVideoTrackNumber(r io.ReaderAt, offset, segmentEnd int64) (uint64, error) {
	payload, err := readEBMLBoundedPayload(r, offset, segmentEnd, tracksID, maxEBMLHeaderPayloadBytes)
	if err != nil {
		return 0, err
	}

	videoTrack := uint64(0)
	walkErr := walkEBMLChildren(payload, func(id uint32, data []byte) error {
		if id != trackEntryID || videoTrack != 0 {
			return nil
		}

		number := uint64(0)
		trackType := uint64(0)
		entryErr := walkEBMLChildren(data, func(childID uint32, childData []byte) error {
			switch childID {
			case trackNumberID:
				number = ebmlUint(childData)
			case trackTypeID:
				trackType = ebmlUint(childData)
			}
			return nil
		})
		if entryErr != nil {
			return entryErr
		}

		if trackType == matroskaVideoTrackType && number != 0 {
			videoTrack = number
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}

	return videoTrack, nil
}

func parseEBMLCues(
	r io.ReaderAt,
	offset, segmentEnd int64,
	videoTrack uint64,
	timestampScale float64,
) ([]float64, error) {
	payload, err := readEBMLBoundedPayload(r, offset, segmentEnd, cuesID, maxCuesPayloadBytes)
	if err != nil {
		return nil, err
	}

	keyframes := make([]float64, 0, 1024)
	walkErr := walkEBMLChildren(payload, func(id uint32, data []byte) error {
		if id != cuePointID {
			return nil
		}

		cueTime := int64(-1)
		matchesTrack := videoTrack == 0
		pointErr := walkEBMLChildren(data, func(childID uint32, childData []byte) error {
			switch childID {
			case cueTimeID:
				cueTime = int64(ebmlUint(childData))
			case cueTrackPosID:
				trackErr := walkEBMLChildren(childData, func(posID uint32, posData []byte) error {
					if posID == cueTrackID && ebmlUint(posData) == videoTrack {
						matchesTrack = true
					}
					return nil
				})
				if trackErr != nil {
					return trackErr
				}
			}
			return nil
		})
		if pointErr != nil {
			return pointErr
		}

		if cueTime >= 0 && matchesTrack {
			keyframes = append(keyframes, float64(cueTime)*timestampScale/1e9)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	if len(keyframes) == 0 {
		return nil, ErrNoIndex
	}
	return keyframes, nil
}

func parseEBMLSeekEntry(data []byte) (uint32, uint64, error) {
	targetID := uint32(0)
	position := uint64(0)
	err := walkEBMLChildren(data, func(id uint32, childData []byte) error {
		switch id {
		case seekEntryIDID:
			// The SeekID payload is the raw bytes of the referenced element ID.
			raw := uint64(0)
			for _, b := range childData {
				raw = raw<<8 | uint64(b)
			}
			if raw <= math.MaxUint32 {
				targetID = uint32(raw)
			}
		case seekPositionID:
			position = ebmlUint(childData)
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return targetID, position, nil
}

// readEBMLBoundedPayload validates the element at offset and reads its whole
// payload in one sequential read.
func readEBMLBoundedPayload(r io.ReaderAt, offset, segmentEnd int64, wantID uint32, maxPayloadBytes int64) ([]byte, error) {
	element, err := readEBMLElementHeader(r, offset, segmentEnd)
	if err != nil {
		return nil, fmt.Errorf("read element 0x%X header: %w", wantID, err)
	}
	if element.ID != wantID {
		return nil, fmt.Errorf("expected element 0x%X at offset %d, found 0x%X", wantID, offset, element.ID)
	}
	if element.DataSize == ebmlUnknownSize {
		return nil, ErrNoIndex
	}
	if element.DataSize > maxPayloadBytes {
		return nil, ErrNoIndex
	}

	payload := make([]byte, element.DataSize)
	_, err = r.ReadAt(payload, element.DataStart)
	if err != nil {
		return nil, fmt.Errorf("read element 0x%X payload: %w", wantID, err)
	}
	return payload, nil
}

// walkEBMLChildren iterates the immediate children of an in-memory payload.
func walkEBMLChildren(payload []byte, visit func(id uint32, data []byte) error) error {
	offset := 0
	for offset < len(payload) {
		id, idLen, err := decodeEBMLID(payload[offset:])
		if err != nil {
			return err
		}
		size, sizeLen, err := decodeEBMLSize(payload[offset+idLen:])
		if err != nil {
			return err
		}
		if size == ebmlUnknownSize {
			return ErrNoIndex
		}

		dataStart := offset + idLen + sizeLen
		dataEnd := dataStart + int(size)
		if dataEnd > len(payload) || dataEnd < dataStart {
			return fmt.Errorf("element 0x%X overruns its parent", id)
		}

		err = visit(id, payload[dataStart:dataEnd])
		if err != nil {
			return err
		}
		offset = dataEnd
	}
	return nil
}

// readEBMLElementHeader reads one element header (ID + size VINT) at an
// absolute file offset without touching the payload.
func readEBMLElementHeader(r io.ReaderAt, offset, limit int64) (ebmlElement, error) {
	if offset < 0 || offset >= limit {
		return ebmlElement{}, fmt.Errorf("element offset %d outside file", offset)
	}

	// An ID is at most 4 bytes and a size VINT at most 8.
	header := make([]byte, 12)
	n, err := r.ReadAt(header, offset)
	if err != nil && err != io.EOF {
		return ebmlElement{}, err
	}
	header = header[:n]

	id, idLen, err := decodeEBMLID(header)
	if err != nil {
		return ebmlElement{}, err
	}
	size, sizeLen, err := decodeEBMLSize(header[idLen:])
	if err != nil {
		return ebmlElement{}, err
	}

	return ebmlElement{
		ID:        id,
		DataStart: offset + int64(idLen) + int64(sizeLen),
		DataSize:  size,
	}, nil
}

// decodeEBMLID decodes an element ID, keeping the length-marker bits as the
// Matroska spec's ID constants do.
func decodeEBMLID(data []byte) (uint32, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("truncated EBML ID")
	}

	first := data[0]
	length := 0
	switch {
	case first&0x80 != 0:
		length = 1
	case first&0x40 != 0:
		length = 2
	case first&0x20 != 0:
		length = 3
	case first&0x10 != 0:
		length = 4
	default:
		return 0, 0, fmt.Errorf("invalid EBML ID leading byte 0x%02X", first)
	}
	if len(data) < length {
		return 0, 0, fmt.Errorf("truncated EBML ID")
	}

	id := uint32(0)
	for i := 0; i < length; i++ {
		id = id<<8 | uint32(data[i])
	}
	return id, length, nil
}

// decodeEBMLSize decodes a size VINT, stripping the marker bit. A value of
// all ones means "unknown size" and returns ebmlUnknownSize.
func decodeEBMLSize(data []byte) (int64, int, error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("truncated EBML size")
	}

	first := data[0]
	length := 0
	for i := 0; i < 8; i++ {
		if first&(0x80>>i) != 0 {
			length = i + 1
			break
		}
	}
	if length == 0 {
		return 0, 0, fmt.Errorf("invalid EBML size leading byte 0x%02X", first)
	}
	if len(data) < length {
		return 0, 0, fmt.Errorf("truncated EBML size")
	}

	value := int64(first &^ (0x80 >> (length - 1)))
	allOnes := value == int64(0x7F>>(length-1))
	for i := 1; i < length; i++ {
		value = value<<8 | int64(data[i])
		if data[i] != 0xFF {
			allOnes = false
		}
	}
	if allOnes {
		return ebmlUnknownSize, length, nil
	}
	return value, length, nil
}

// ebmlUint reads a 0-8 byte big-endian unsigned integer payload.
func ebmlUint(data []byte) uint64 {
	if len(data) > 8 {
		return 0
	}
	value := uint64(0)
	for _, b := range data {
		value = value<<8 | uint64(b)
	}
	return value
}

// ebmlFloat reads a 4- or 8-byte big-endian IEEE float payload.
func ebmlFloat(data []byte) (float64, bool) {
	switch len(data) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(data))), true
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(data)), true
	default:
		return 0, false
	}
}
