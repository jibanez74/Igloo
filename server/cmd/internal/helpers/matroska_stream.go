package helpers

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"time"
)

// Top-level Matroska element IDs, with their length marker bits kept.
const (
	matroskaEBMLID    = 0x1A45DFA3
	matroskaSegmentID = 0x18538067
	matroskaTracksID  = 0x1654AE6B
	matroskaClusterID = 0x1F43B675
)

// maxMatroskaHeaderBytes bounds the header read before the first Cluster;
// FFmpeg's single-track subtitle header is under a kilobyte.
const maxMatroskaHeaderBytes = 1 << 20

// matroskaRelayChunkBytes bounds one relayed piece of an element's payload.
const matroskaRelayChunkBytes = 64 << 10

// MatroskaPrimerCluster is an empty Cluster at timestamp 0: ID, a one-byte
// size of 3, then a Timestamp element holding 0. A player's demuxer only
// counts a non-seekable Matroska stream as ready once it reaches a Cluster,
// and FFmpeg writes its first one only when it has the track's first packet,
// which can lie minutes into the source.
var MatroskaPrimerCluster = []byte{0x1F, 0x43, 0xB6, 0x75, 0x83, 0xE7, 0x81, 0x00}

// matroskaVoid is a Void element with one byte of padding, written to keep a
// silent stream's connection alive between elements.
var matroskaVoid = []byte{0xEC, 0x81, 0x00}

// ErrMatroskaHeader reports a stream whose header could not be primed.
var ErrMatroskaHeader = errors.New("unexpected matroska header")

// ErrMatroskaRelayWrite marks a relay that failed writing to its
// destination, as opposed to reading a source that broke off or went bad.
var ErrMatroskaRelayWrite = errors.New("write matroska stream")

// ReadMatroskaHeader reads a Matroska stream through its Tracks element — the
// EBML header, the Segment's ID and size, and the Segment's children up to
// Tracks — and returns those bytes with MatroskaPrimerCluster appended. The
// elements after Tracks stay in r for RelayMatroskaElements. Any child FFmpeg
// writes after Tracks, such as Tags, follows the primer, which Matroska
// allows.
func ReadMatroskaHeader(r *bufio.Reader) ([]byte, error) {
	header := make([]byte, 0, 1024)

	id, size, unknownSize, raw, err := readMatroskaElementHeader(r)
	if err != nil {
		return nil, err
	}
	isEBMLHeader := id == matroskaEBMLID && !unknownSize
	if !isEBMLHeader {
		return nil, fmt.Errorf("%w: stream does not start with an EBML header", ErrMatroskaHeader)
	}
	header, err = appendMatroskaElement(header, r, raw, size)
	if err != nil {
		return nil, err
	}

	id, _, _, raw, err = readMatroskaElementHeader(r)
	if err != nil {
		return nil, err
	}
	if id != matroskaSegmentID {
		return nil, fmt.Errorf("%w: no Segment after the EBML header", ErrMatroskaHeader)
	}
	header = append(header, raw...)

	for {
		id, size, unknownSize, raw, err = readMatroskaElementHeader(r)
		if err != nil {
			return nil, err
		}
		if id == matroskaClusterID {
			return nil, fmt.Errorf("%w: Cluster before Tracks", ErrMatroskaHeader)
		}
		if unknownSize {
			return nil, fmt.Errorf("%w: element %#x of unknown size before Tracks", ErrMatroskaHeader, id)
		}
		header, err = appendMatroskaElement(header, r, raw, size)
		if err != nil {
			return nil, err
		}
		if id == matroskaTracksID {
			return append(header, MatroskaPrimerCluster...), nil
		}
	}
}

// RelayMatroskaElements copies the elements that follow a primed header from
// src to dst, flushing after each write. While src is silent for keepalive at
// an element boundary, it writes a Void element, so a sparse track keeps its
// connection alive while FFmpeg reads ahead to the next cue. An element of
// unknown size ends the keepalives; the rest is copied as it comes. It
// returns nil when src ends at an element boundary. The caller must end src
// once it returns, since a read may still be pending on it.
func RelayMatroskaElements(src *bufio.Reader, dst io.Writer, flush func() error, keepalive time.Duration) error {
	pieces := make(chan matroskaPiece)
	done := make(chan struct{})
	defer close(done)
	go readMatroskaPieces(src, pieces, done)

	timer := time.NewTimer(keepalive)
	defer timer.Stop()
	atBoundary := true

	for {
		select {
		case piece := <-pieces:
			if piece.err != nil {
				srcEnded := errors.Is(piece.err, io.EOF)
				if srcEnded && atBoundary {
					return nil
				}
				if srcEnded {
					return io.ErrUnexpectedEOF
				}
				return piece.err
			}
			err := writeAndFlush(dst, flush, piece.data)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrMatroskaRelayWrite, err)
			}
			atBoundary = piece.boundary
			timer.Reset(keepalive)
		case <-timer.C:
			if atBoundary {
				err := writeAndFlush(dst, flush, matroskaVoid)
				if err != nil {
					return fmt.Errorf("%w: %w", ErrMatroskaRelayWrite, err)
				}
			}
			timer.Reset(keepalive)
		}
	}
}

// matroskaPiece is part of an element, or the read error that ended the
// stream. boundary says the piece ends an element.
type matroskaPiece struct {
	data     []byte
	boundary bool
	err      error
}

func readMatroskaPieces(src *bufio.Reader, pieces chan<- matroskaPiece, done <-chan struct{}) {
	send := func(piece matroskaPiece) bool {
		select {
		case pieces <- piece:
			return true
		case <-done:
			return false
		}
	}

	for {
		_, size, unknownSize, raw, err := readMatroskaElementHeader(src)
		if err != nil {
			send(matroskaPiece{err: err})
			return
		}
		if unknownSize {
			sent := send(matroskaPiece{data: raw})
			if sent {
				relayRawPieces(src, send)
			}
			return
		}
		sent := send(matroskaPiece{data: raw, boundary: size == 0})
		if !sent {
			return
		}

		remaining := size
		for remaining > 0 {
			chunkSize := min(remaining, matroskaRelayChunkBytes)
			chunk := make([]byte, chunkSize)
			_, err = io.ReadFull(src, chunk)
			if err != nil {
				send(matroskaPiece{err: unexpectedEOF(err)})
				return
			}
			remaining -= chunkSize
			sent = send(matroskaPiece{data: chunk, boundary: remaining == 0})
			if !sent {
				return
			}
		}
	}
}

// relayRawPieces copies the rest of src as it arrives; nothing after an
// element of unknown size is ever at a known boundary.
func relayRawPieces(src *bufio.Reader, send func(matroskaPiece) bool) {
	for {
		chunk := make([]byte, matroskaRelayChunkBytes)
		n, err := src.Read(chunk)
		if n > 0 {
			sent := send(matroskaPiece{data: chunk[:n]})
			if !sent {
				return
			}
		}
		if err != nil {
			// Ending inside an element of unknown size is an unexpected end.
			send(matroskaPiece{err: unexpectedEOF(err)})
			return
		}
	}
}

func writeAndFlush(dst io.Writer, flush func() error, data []byte) error {
	_, err := dst.Write(data)
	if err != nil {
		return err
	}
	return flush()
}

// appendMatroskaElement appends an element's header bytes and its size bytes
// of payload read from r.
func appendMatroskaElement(header []byte, r *bufio.Reader, raw []byte, size uint64) ([]byte, error) {
	total := uint64(len(header)) + uint64(len(raw)) + size
	if total > maxMatroskaHeaderBytes {
		return nil, fmt.Errorf("%w: header exceeds %d bytes", ErrMatroskaHeader, maxMatroskaHeaderBytes)
	}
	header = append(header, raw...)
	start := len(header)
	header = append(header, make([]byte, size)...)
	_, err := io.ReadFull(r, header[start:])
	if err != nil {
		return nil, unexpectedEOF(err)
	}
	return header, nil
}

// readMatroskaElementHeader reads one element's ID, whose length marker bits
// are kept, and its size, and returns the raw bytes of both. An all-ones size
// is the unknown size of a streamed master element.
func readMatroskaElementHeader(r *bufio.Reader) (id uint32, size uint64, unknownSize bool, raw []byte, err error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, 0, false, nil, err
	}
	idLength := matroskaVintLength(first)
	if idLength == 0 || idLength > 4 {
		return 0, 0, false, nil, fmt.Errorf("%w: invalid element ID byte %#x", ErrMatroskaHeader, first)
	}
	raw = append(raw, first)
	id = uint32(first)
	for range idLength - 1 {
		next, readErr := r.ReadByte()
		if readErr != nil {
			return 0, 0, false, nil, unexpectedEOF(readErr)
		}
		raw = append(raw, next)
		id = id<<8 | uint32(next)
	}

	first, err = r.ReadByte()
	if err != nil {
		return 0, 0, false, nil, unexpectedEOF(err)
	}
	sizeLength := matroskaVintLength(first)
	if sizeLength == 0 {
		return 0, 0, false, nil, fmt.Errorf("%w: invalid element size byte %#x", ErrMatroskaHeader, first)
	}
	raw = append(raw, first)
	marker := byte(0x80) >> (sizeLength - 1)
	size = uint64(first &^ marker)
	allOnes := first&^marker == marker-1
	for range sizeLength - 1 {
		next, readErr := r.ReadByte()
		if readErr != nil {
			return 0, 0, false, nil, unexpectedEOF(readErr)
		}
		raw = append(raw, next)
		size = size<<8 | uint64(next)
		allOnes = allOnes && next == 0xFF
	}
	return id, size, allOnes, raw, nil
}

// matroskaVintLength is the byte length an EBML variable-length integer
// declares in its first byte's leading zeros, or 0 for a zero byte.
func matroskaVintLength(first byte) int {
	for length := 1; length <= 8; length++ {
		marker := byte(0x80) >> (length - 1)
		hasMarker := first&marker != 0
		if hasMarker {
			return length
		}
	}
	return 0
}

func unexpectedEOF(err error) error {
	isEOF := errors.Is(err, io.EOF)
	if isEOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
