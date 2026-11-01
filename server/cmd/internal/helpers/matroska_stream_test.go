package helpers

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"
	"testing/iotest"
	"time"
)

var (
	testEBMLID     = []byte{0x1A, 0x45, 0xDF, 0xA3}
	testSegmentID  = []byte{0x18, 0x53, 0x80, 0x67}
	testSeekHeadID = []byte{0x11, 0x4D, 0x9B, 0x74}
	testInfoID     = []byte{0x15, 0x49, 0xA9, 0x66}
	testTracksID   = []byte{0x16, 0x54, 0xAE, 0x6B}
	testTagsID     = []byte{0x12, 0x54, 0xC3, 0x67}
	testClusterID  = []byte{0x1F, 0x43, 0xB6, 0x75}
	testVoidID     = []byte{0xEC}

	// testUnknownSize is the 8-byte all-ones size FFmpeg gives a streamed
	// Segment.
	testUnknownSize = []byte{0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
)

// testElement encodes an element with a one-byte size up to 126 bytes of
// payload and a two-byte size up to 16382.
func testElement(id []byte, payload []byte) []byte {
	out := slices.Clone(id)
	if len(payload) < 0x7F {
		out = append(out, 0x80|byte(len(payload)))
	} else {
		out = append(out, 0x40|byte(len(payload)>>8), byte(len(payload)))
	}
	return append(out, payload...)
}

func testCluster(timestamp byte, block []byte) []byte {
	payload := append([]byte{0xE7, 0x81, timestamp}, testElement([]byte{0xA3}, block)...)
	return testElement(testClusterID, payload)
}

// testMatroskaStream lays out a single-track stream the way FFmpeg writes one
// to a pipe: EBML header, Segment of unknown size, SeekHead, Void, Info,
// Tracks, Tags, then Clusters.
type testMatroskaStream struct {
	beforeTracks []byte
	tracks       []byte
	afterTracks  []byte
}

func newTestMatroskaStream() testMatroskaStream {
	var before []byte
	before = append(before, testElement(testEBMLID, []byte{0x42, 0x82, 0x88, 'm', 'a', 't', 'r', 'o', 's', 'k', 'a'})...)
	before = append(before, testSegmentID...)
	before = append(before, testUnknownSize...)
	before = append(before, testElement(testSeekHeadID, bytes.Repeat([]byte{0x4D}, 20))...)
	before = append(before, testElement(testVoidID, make([]byte, 30))...)
	before = append(before, testElement(testInfoID, []byte{0x2A, 0xD7, 0xB1, 0x83, 0x0F, 0x42, 0x40})...)

	tracks := testElement(testTracksID, testElement([]byte{0xAE}, []byte{0xD7, 0x81, 0x01, 0x86, 0x8A, 'S', '_', 'H', 'D', 'M', 'V', '/', 'P', 'G', 'S'}))

	var after []byte
	after = append(after, testElement(testTagsID, []byte("encoder"))...)
	after = append(after, testCluster(1, []byte{0x81, 0x00, 0x00, 0x80, 0xAA})...)
	after = append(after, testCluster(4, bytes.Repeat([]byte{0xBB}, 300))...)

	return testMatroskaStream{beforeTracks: before, tracks: tracks, afterTracks: after}
}

func (s testMatroskaStream) bytes() []byte {
	out := slices.Concat(s.beforeTracks, s.tracks, s.afterTracks)
	return out
}

func TestReadMatroskaHeaderAppendsPrimerAfterTracks(t *testing.T) {
	t.Parallel()

	stream := newTestMatroskaStream()
	readers := map[string]io.Reader{
		"whole":        bytes.NewReader(stream.bytes()),
		"one byte":     iotest.OneByteReader(bytes.NewReader(stream.bytes())),
		"half read":    iotest.HalfReader(bytes.NewReader(stream.bytes())),
		"data err EOF": iotest.DataErrReader(bytes.NewReader(stream.bytes())),
	}

	for name, reader := range readers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			src := bufio.NewReader(reader)
			header, err := ReadMatroskaHeader(src)
			if err != nil {
				t.Fatalf("ReadMatroskaHeader: %v", err)
			}
			want := slices.Concat(stream.beforeTracks, stream.tracks, MatroskaPrimerCluster)
			if !bytes.Equal(header, want) {
				t.Fatalf("header = %x\nwant     %x", header, want)
			}
			rest, err := io.ReadAll(src)
			if err != nil {
				t.Fatalf("read rest: %v", err)
			}
			if !bytes.Equal(rest, stream.afterTracks) {
				t.Fatalf("rest = %x, want %x", rest, stream.afterTracks)
			}
		})
	}
}

// The primer must decode as an empty Cluster whose only child is a zero
// Timestamp; a player that reads it as anything else never prepares.
func TestMatroskaPrimerClusterIsAnEmptyClusterAtZero(t *testing.T) {
	t.Parallel()

	src := bufio.NewReader(bytes.NewReader(MatroskaPrimerCluster))
	id, size, unknownSize, raw, err := readMatroskaElementHeader(src)
	if err != nil {
		t.Fatalf("read primer header: %v", err)
	}
	if id != matroskaClusterID || size != 3 || unknownSize || len(raw) != 5 {
		t.Fatalf("primer header = id %#x size %d unknown %v raw %x", id, size, unknownSize, raw)
	}
	id, size, _, _, err = readMatroskaElementHeader(src)
	if err != nil {
		t.Fatalf("read timestamp header: %v", err)
	}
	value, err := src.ReadByte()
	if err != nil {
		t.Fatalf("read timestamp value: %v", err)
	}
	if id != 0xE7 || size != 1 || value != 0 {
		t.Fatalf("timestamp = id %#x size %d value %d, want 0xe7 1 0", id, size, value)
	}
}

func TestReadMatroskaHeaderAcceptsKnownSizeSegment(t *testing.T) {
	t.Parallel()

	stream := newTestMatroskaStream()
	segmentAt := bytes.Index(stream.beforeTracks, testSegmentID)
	knownSize := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00}
	before := slices.Concat(stream.beforeTracks[:segmentAt+4], knownSize, stream.beforeTracks[segmentAt+4+len(testUnknownSize):])
	input := slices.Concat(before, stream.tracks, stream.afterTracks)

	header, err := ReadMatroskaHeader(bufio.NewReader(bytes.NewReader(input)))
	if err != nil {
		t.Fatalf("ReadMatroskaHeader: %v", err)
	}
	want := slices.Concat(before, stream.tracks, MatroskaPrimerCluster)
	if !bytes.Equal(header, want) {
		t.Fatalf("header = %x, want %x", header, want)
	}
}

func TestReadMatroskaHeaderRejectsStreamsItCannotPrime(t *testing.T) {
	t.Parallel()

	stream := newTestMatroskaStream()
	ebml := testElement(testEBMLID, []byte{0x42, 0x82, 0x81, 'x'})
	segment := slices.Concat(testSegmentID, testUnknownSize)
	info := testElement(testInfoID, []byte{0x01})

	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "empty", input: nil, wantErr: io.EOF},
		{name: "not EBML", input: info, wantErr: ErrMatroskaHeader},
		{name: "EBML of unknown size", input: slices.Concat(testEBMLID, testUnknownSize), wantErr: ErrMatroskaHeader},
		{name: "no Segment", input: slices.Concat(ebml, info), wantErr: ErrMatroskaHeader},
		{name: "Cluster before Tracks", input: slices.Concat(ebml, segment, info, testCluster(0, []byte{1})), wantErr: ErrMatroskaHeader},
		{name: "unknown size before Tracks", input: slices.Concat(ebml, segment, testInfoID, testUnknownSize), wantErr: ErrMatroskaHeader},
		{name: "invalid ID byte", input: slices.Concat(ebml, segment, []byte{0x00, 0x81, 0x00}), wantErr: ErrMatroskaHeader},
		{name: "five-byte ID", input: slices.Concat(ebml, segment, []byte{0x08, 0x01, 0x02, 0x03, 0x04, 0x81, 0x00}), wantErr: ErrMatroskaHeader},
		{name: "invalid size byte", input: slices.Concat(ebml, segment, testInfoID, []byte{0x00}), wantErr: ErrMatroskaHeader},
		{name: "ends inside an ID", input: slices.Concat(ebml, segment, testInfoID[:2]), wantErr: io.ErrUnexpectedEOF},
		{name: "ends inside a payload", input: slices.Concat(ebml, segment, testInfoID, []byte{0x85, 0x01}), wantErr: io.ErrUnexpectedEOF},
		{name: "ends before Tracks", input: stream.beforeTracks, wantErr: io.EOF},
		{
			name:    "header too large",
			input:   slices.Concat(ebml, segment, testInfoID, []byte{0x10, 0x20, 0x00, 0x00}),
			wantErr: ErrMatroskaHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ReadMatroskaHeader(bufio.NewReader(bytes.NewReader(tt.input)))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReadMatroskaHeader error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// relayRecorder collects relayed bytes and counts flushes; the relay writes
// from its own goroutine while tests read.
type relayRecorder struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	writes  int
	flushes int
	err     error
}

func (r *relayRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	r.writes++
	return r.buf.Write(p)
}

func (r *relayRecorder) flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushes++
	return nil
}

func (r *relayRecorder) bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.buf.Bytes())
}

func (r *relayRecorder) counts() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writes, r.flushes
}

func TestRelayMatroskaElementsCopiesElementsAndFlushesEachWrite(t *testing.T) {
	t.Parallel()

	stream := newTestMatroskaStream()
	recorder := &relayRecorder{}

	err := RelayMatroskaElements(bufio.NewReader(bytes.NewReader(stream.afterTracks)), recorder, recorder.flush, time.Hour)
	if err != nil {
		t.Fatalf("RelayMatroskaElements: %v", err)
	}
	if !bytes.Equal(recorder.bytes(), stream.afterTracks) {
		t.Fatalf("relayed = %x, want %x", recorder.bytes(), stream.afterTracks)
	}
	writes, flushes := recorder.counts()
	if writes == 0 || writes != flushes {
		t.Fatalf("writes = %d, flushes = %d, want every write flushed", writes, flushes)
	}
}

func TestRelayMatroskaElementsSplitsLargePayloads(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte{0x5A}, matroskaRelayChunkBytes*2+17)
	sizeBytes := []byte{0x01, 0x00, 0x00, 0x00, 0x00, byte(len(payload) >> 16), byte(len(payload) >> 8), byte(len(payload))}
	element := slices.Concat(testClusterID, sizeBytes, payload)
	recorder := &relayRecorder{}

	err := RelayMatroskaElements(bufio.NewReader(bytes.NewReader(element)), recorder, recorder.flush, time.Hour)
	if err != nil {
		t.Fatalf("RelayMatroskaElements: %v", err)
	}
	if !bytes.Equal(recorder.bytes(), element) {
		t.Fatal("large element was not relayed byte for byte")
	}
	writes, _ := recorder.counts()
	if writes != 4 {
		t.Fatalf("writes = %d, want the header and three payload chunks", writes)
	}
}

func TestRelayMatroskaElementsReportsAStreamCutInsideAnElement(t *testing.T) {
	t.Parallel()

	cluster := testCluster(2, []byte{1, 2, 3, 4})
	inputs := map[string][]byte{
		"inside the header":  cluster[:2],
		"inside the payload": cluster[:len(cluster)-2],
	}

	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := RelayMatroskaElements(bufio.NewReader(bytes.NewReader(input)), &relayRecorder{}, func() error { return nil }, time.Hour)
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("RelayMatroskaElements error = %v, want io.ErrUnexpectedEOF", err)
			}
		})
	}
}

func TestRelayMatroskaElementsReturnsReadAndWriteErrors(t *testing.T) {
	t.Parallel()

	// Only a failure on the destination side carries ErrMatroskaRelayWrite:
	// the caller tells a broken source from a player that stopped reading.
	readFailure := errors.New("pipe broke")
	err := RelayMatroskaElements(bufio.NewReader(iotest.ErrReader(readFailure)), &relayRecorder{}, func() error { return nil }, time.Hour)
	if !errors.Is(err, readFailure) || errors.Is(err, ErrMatroskaRelayWrite) {
		t.Fatalf("read error = %v, want %v alone", err, readFailure)
	}

	stream := newTestMatroskaStream()
	cut := stream.afterTracks[:len(stream.afterTracks)-1]
	err = RelayMatroskaElements(bufio.NewReader(bytes.NewReader(cut)), &relayRecorder{}, func() error { return nil }, time.Hour)
	if !errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ErrMatroskaRelayWrite) {
		t.Fatalf("cut source error = %v, want io.ErrUnexpectedEOF alone", err)
	}

	writeFailure := errors.New("client gone")
	recorder := &relayRecorder{err: writeFailure}
	err = RelayMatroskaElements(bufio.NewReader(bytes.NewReader(stream.afterTracks)), recorder, recorder.flush, time.Hour)
	if !errors.Is(err, writeFailure) || !errors.Is(err, ErrMatroskaRelayWrite) {
		t.Fatalf("write error = %v, want %v marked as a write", err, writeFailure)
	}

	flushFailure := errors.New("flush failed")
	err = RelayMatroskaElements(bufio.NewReader(bytes.NewReader(stream.afterTracks)), &relayRecorder{}, func() error { return flushFailure }, time.Hour)
	if !errors.Is(err, flushFailure) || !errors.Is(err, ErrMatroskaRelayWrite) {
		t.Fatalf("flush error = %v, want %v marked as a write", err, flushFailure)
	}
}

// waitForRelayed polls until the recorder holds want or the deadline passes.
func waitForRelayed(t *testing.T, recorder *relayRecorder, want []byte) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		matched := bytes.Equal(recorder.bytes(), want)
		if matched {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("relayed = %x\nwant      %x", recorder.bytes(), want)
}

// A silent source gets Void elements only between elements: one inside a
// half-written Cluster would corrupt it.
func TestRelayMatroskaElementsKeepsSilenceAliveOnlyAtBoundaries(t *testing.T) {
	t.Parallel()

	const keepalive = 20 * time.Millisecond
	pipeReader, pipeWriter := io.Pipe()
	recorder := &relayRecorder{}
	relayDone := make(chan error, 1)
	go func() {
		relayDone <- RelayMatroskaElements(bufio.NewReader(pipeReader), recorder, recorder.flush, keepalive)
	}()

	first := testCluster(1, []byte{0xAA})
	_, err := pipeWriter.Write(first)
	if err != nil {
		t.Fatalf("write first cluster: %v", err)
	}
	waitForRelayed(t, recorder, slices.Concat(first, matroskaVoid))

	// The relay passes an element's header on at once and holds a partial
	// payload chunk until it is complete.
	second := testCluster(9, []byte{0xBB, 0xCC})
	split := len(second) - 2
	secondHeader := second[:len(testClusterID)+1]
	before := recorder.bytes()
	_, err = pipeWriter.Write(second[:split])
	if err != nil {
		t.Fatalf("write half a cluster: %v", err)
	}
	waitForRelayed(t, recorder, slices.Concat(before, secondHeader))
	time.Sleep(10 * keepalive)
	midElement := recorder.bytes()
	if !bytes.Equal(midElement, slices.Concat(before, secondHeader)) {
		t.Fatalf("relay wrote inside an element: %x", midElement[len(before)+len(secondHeader):])
	}

	_, err = pipeWriter.Write(second[split:])
	if err != nil {
		t.Fatalf("finish the cluster: %v", err)
	}
	err = pipeWriter.Close()
	if err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	err = <-relayDone
	if err != nil {
		t.Fatalf("RelayMatroskaElements: %v", err)
	}

	relayed := recorder.bytes()
	withoutVoids := bytes.ReplaceAll(relayed, matroskaVoid, nil)
	if !bytes.Equal(withoutVoids, slices.Concat(first, second)) {
		t.Fatalf("relayed without voids = %x, want both clusters", withoutVoids)
	}
	voidAt := bytes.Index(relayed, matroskaVoid)
	if voidAt != len(first) {
		t.Fatalf("first Void at %d, want right after the first cluster at %d", voidAt, len(first))
	}
}

// After an element of unknown size nothing is at a known boundary, so the
// rest is copied as it comes and no Void is ever written.
func TestRelayMatroskaElementsCopiesRawAfterAnUnknownSize(t *testing.T) {
	t.Parallel()

	const keepalive = 10 * time.Millisecond
	pipeReader, pipeWriter := io.Pipe()
	recorder := &relayRecorder{}
	relayDone := make(chan error, 1)
	go func() {
		relayDone <- RelayMatroskaElements(bufio.NewReader(pipeReader), recorder, recorder.flush, keepalive)
	}()

	unknownCluster := slices.Concat(testClusterID, testUnknownSize, []byte{0xE7, 0x81, 0x00})
	_, err := pipeWriter.Write(unknownCluster)
	if err != nil {
		t.Fatalf("write cluster: %v", err)
	}
	waitForRelayed(t, recorder, unknownCluster)
	time.Sleep(10 * keepalive)
	trailing := []byte{0xA3, 0x81, 0x01}
	_, err = pipeWriter.Write(trailing)
	if err != nil {
		t.Fatalf("write trailing bytes: %v", err)
	}
	waitForRelayed(t, recorder, slices.Concat(unknownCluster, trailing))
	err = pipeWriter.Close()
	if err != nil {
		t.Fatalf("close pipe: %v", err)
	}

	err = <-relayDone
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("RelayMatroskaElements error = %v, want the cut inside an element of unknown size", err)
	}
	if bytes.Contains(recorder.bytes(), matroskaVoid) {
		t.Fatalf("relay wrote a Void after an element of unknown size: %x", recorder.bytes())
	}
}
