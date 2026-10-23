// Package isobmff walks the box structure of ISO base media files (mp4, m4a,
// m4v, mov). It reads the moov box wherever the muxer put it and lists boxes
// inside an in-memory payload; callers interpret the boxes they need.
package isobmff

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MaxMoovPayloadBytes caps the moov read. A corrupt or hostile file must cost
// a bounded read and a clean fallback, never an OOM.
const MaxMoovPayloadBytes = 32 << 20

// ErrMoovTooLarge means the file's moov box exceeds MaxMoovPayloadBytes.
var ErrMoovTooLarge = errors.New("isobmff: moov box too large")

// ReadMoov walks top-level boxes to find moov wherever the muxer put it and
// reads its payload with a single sequential read.
func ReadMoov(ctx context.Context, r io.ReaderAt, size int64) ([]byte, error) {
	offset := int64(0)
	for offset < size {
		err := ctx.Err()
		if err != nil {
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
			if payloadSize > MaxMoovPayloadBytes {
				return nil, ErrMoovTooLarge
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

// Box delimits one box inside an in-memory payload.
type Box struct {
	Type         string
	PayloadStart int
	End          int
}

// ChildBoxes lists the boxes between start and end of data.
func ChildBoxes(data []byte, start, end int) ([]Box, error) {
	boxes := make([]Box, 0, 8)
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

		boxes = append(boxes, Box{
			Type:         boxType,
			PayloadStart: payloadStart,
			End:          offset + size,
		})
		offset += size
	}
	return boxes, nil
}

// FindChild returns the first box of boxType between start and end of data.
func FindChild(data []byte, start, end int, boxType string) (Box, bool, error) {
	boxes, err := ChildBoxes(data, start, end)
	if err != nil {
		return Box{}, false, err
	}
	for _, box := range boxes {
		if box.Type == boxType {
			return box, true, nil
		}
	}
	return Box{}, false, nil
}
