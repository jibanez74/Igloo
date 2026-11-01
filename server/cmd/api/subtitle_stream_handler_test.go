package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

// A minimal stream the way FFmpeg writes one to a pipe: EBML header, a
// Segment of unknown size, Info, Tracks, then a Cluster.
var (
	testStreamHeader = bytes.Join([][]byte{
		{0x1A, 0x45, 0xDF, 0xA3, 0x81, 0x00},
		{0x18, 0x53, 0x80, 0x67, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		{0x15, 0x49, 0xA9, 0x66, 0x81, 0x00},
		{0x16, 0x54, 0xAE, 0x6B, 0x81, 0x00},
	}, nil)
	testStreamCluster = []byte{0x1F, 0x43, 0xB6, 0x75, 0x86, 0xE7, 0x81, 0x05, 0xA3, 0x81, 0x01}
	testStreamVoid    = []byte{0xEC, 0x81, 0x00}
)

func testPrimedStream(clusters ...[]byte) []byte {
	return bytes.Join(append([][]byte{testStreamHeader, helpers.MatroskaPrimerCluster}, clusters...), nil)
}

// scriptedSubtitleStream stands in for FFmpeg streaming a track: the test
// writes its output and ends it, and its context ending closes the output the
// way a killed process does.
type scriptedSubtitleStream struct {
	params ffmpeg.SubtitleStreamParams
	ctx    context.Context
	reader *io.PipeReader
	writer *io.PipeWriter
	once   sync.Once
	ended  chan struct{}
	err    error
}

func newScriptedSubtitleStream(ctx context.Context, params ffmpeg.SubtitleStreamParams) *scriptedSubtitleStream {
	reader, writer := io.Pipe()
	stream := &scriptedSubtitleStream{
		params: params,
		ctx:    ctx,
		reader: reader,
		writer: writer,
		ended:  make(chan struct{}),
	}
	go func() {
		select {
		case <-ctx.Done():
			stream.end(ctx.Err())
		case <-stream.ended:
		}
	}()
	return stream
}

func (s *scriptedSubtitleStream) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *scriptedSubtitleStream) write(t *testing.T, data []byte) {
	t.Helper()
	_, err := s.writer.Write(data)
	if err != nil {
		t.Fatalf("write scripted subtitle stream: %v", err)
	}
}

// end closes the output and makes Wait report exitErr.
func (s *scriptedSubtitleStream) end(exitErr error) {
	s.once.Do(func() {
		s.err = exitErr
		_ = s.writer.Close()
		close(s.ended)
	})
}

func (s *scriptedSubtitleStream) Wait() error {
	<-s.ended
	return s.err
}

// writingSubtitleStreams answers every stream with output, then exitErr.
func writingSubtitleStreams(output []byte, exitErr error) func(context.Context, ffmpeg.SubtitleStreamParams) (ffmpeg.SubtitleStream, error) {
	return func(ctx context.Context, params ffmpeg.SubtitleStreamParams) (ffmpeg.SubtitleStream, error) {
		stream := newScriptedSubtitleStream(ctx, params)
		go func() {
			_, _ = stream.writer.Write(output)
			stream.end(exitErr)
		}()
		return stream, nil
	}
}

// heldSubtitleStreams hands every stream to the test, which writes and ends
// it.
func heldSubtitleStreams(streams chan<- *scriptedSubtitleStream) func(context.Context, ffmpeg.SubtitleStreamParams) (ffmpeg.SubtitleStream, error) {
	return func(ctx context.Context, params ffmpeg.SubtitleStreamParams) (ffmpeg.SubtitleStream, error) {
		stream := newScriptedSubtitleStream(ctx, params)
		streams <- stream
		return stream, nil
	}
}

func receiveStream(t *testing.T, streams <-chan *scriptedSubtitleStream) *scriptedSubtitleStream {
	t.Helper()
	select {
	case stream := <-streams:
		return stream
	case <-time.After(5 * time.Second):
		t.Fatal("no subtitle stream was started")
		return nil
	}
}

func serveSubtitleStream(t *testing.T, app *Application, userID int64, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	authenticatedRouter(t, app, userID).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func movieSubtitleStreamPath(movieID int64, trackIndex string, query string) string {
	return fmt.Sprintf("/api/movies/%d/subtitles/%s/stream.mkv%s", movieID, trackIndex, query)
}

// subtitleStreamServer serves the router over real HTTP, which the
// connection-level behavior — a client leaving, a cut stream — needs.
func subtitleStreamServer(t *testing.T, app *Application, userID int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(authenticatedRouter(t, app, userID))
	t.Cleanup(server.Close)
	return server
}

func overrideSubtitleStreamTiming(t *testing.T, keepalive time.Duration, slotWait time.Duration, cacheGate time.Duration) {
	t.Helper()
	originalKeepalive, originalSlotWait, originalGate := subtitleStreamKeepalive, subtitleStreamSlotWait, subtitleTrackCacheGate
	subtitleStreamKeepalive, subtitleStreamSlotWait, subtitleTrackCacheGate = keepalive, slotWait, cacheGate
	t.Cleanup(func() {
		subtitleStreamKeepalive, subtitleStreamSlotWait, subtitleTrackCacheGate = originalKeepalive, originalSlotWait, originalGate
	})
}

// insertSubtitleStreamMovie inserts a movie whose source file exists on disk,
// so the track cache can fingerprint it, with one subtitle of codec.
func insertSubtitleStreamMovie(t *testing.T, app *Application, container string, codec string) (int64, string) {
	t.Helper()
	sourcePath := filepath.Join(t.TempDir(), "movie source."+container)
	err := os.WriteFile(sourcePath, []byte("media bytes"), 0o644)
	if err != nil {
		t.Fatalf("write source: %v", err)
	}
	movieID := insertTestHLSMovieFixtureAt(t, app, "h264", 1080, sourcePath, container)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, codec, "eng")
	return movieID, sourcePath
}

func TestSubtitleStream_ServesThePrimedTrackFromTheMediaFile(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{streamFunc: writingSubtitleStreams(append(slices.Clone(testStreamHeader), testStreamCluster...), nil)}
	app.FFmpeg = fake
	movieID, sourcePath := insertSubtitleStreamMovie(t, app, "mkv", "subrip")

	recorder := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", "?start=600.5"))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "video/x-matroska" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers = %v", recorder.Header())
	}
	if !bytes.Equal(recorder.Body.Bytes(), testPrimedStream(testStreamCluster)) {
		t.Fatalf("body = %x\nwant  %x", recorder.Body.Bytes(), testPrimedStream(testStreamCluster))
	}
	calls := fake.StreamCalls()
	want := ffmpeg.SubtitleStreamParams{InputPath: sourcePath, StreamIndex: testSubtitleStreamIndex, StartSec: 600.5}
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("stream calls = %+v, want [%+v]", calls, want)
	}
	if len(app.SubtitleStreamSlots) != 0 {
		t.Fatalf("%d stream slots still held after the stream ended", len(app.SubtitleStreamSlots))
	}
}

func TestSubtitleStream_CopiesOrConvertsByCodec(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	tests := []struct {
		codec       string
		container   string
		wantConvert bool
	}{
		{codec: "hdmv_pgs_subtitle", container: "mkv"},
		{codec: "dvd_subtitle", container: "mkv"},
		{codec: "ass", container: "mkv"},
		{codec: "mov_text", container: "mp4", wantConvert: true},
		{codec: "eia_608", container: "mp4", wantConvert: true},
	}

	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			app := setupTestApp(t)
			viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
			fake := &fakeFFmpeg{streamFunc: writingSubtitleStreams(testStreamHeader, nil)}
			app.FFmpeg = fake
			movieID, _ := insertSubtitleStreamMovie(t, app, tt.container, tt.codec)

			recorder := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))

			calls := fake.StreamCalls()
			if recorder.Code != http.StatusOK || len(calls) != 1 {
				t.Fatalf("status = %d, stream calls = %d", recorder.Code, len(calls))
			}
			if calls[0].ConvertToSubRip != tt.wantConvert {
				t.Fatalf("ConvertToSubRip = %v, want %v", calls[0].ConvertToSubRip, tt.wantConvert)
			}
		})
	}
}

func TestSubtitleStream_RejectsWhatItCannotStream(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake
	vobSubInMP4, _ := insertSubtitleStreamMovie(t, app, "mp4", "dvd_subtitle")
	xsub, _ := insertSubtitleStreamMovie(t, app, "avi", "xsub")
	subrip, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")

	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "VobSub outside Matroska", path: movieSubtitleStreamPath(vobSubInMP4, "0", ""), want: http.StatusUnsupportedMediaType},
		{name: "XSUB", path: movieSubtitleStreamPath(xsub, "0", ""), want: http.StatusUnsupportedMediaType},
		{name: "malformed track", path: movieSubtitleStreamPath(subrip, "x", ""), want: http.StatusBadRequest},
		{name: "track out of range", path: movieSubtitleStreamPath(subrip, "1", ""), want: http.StatusBadRequest},
		{name: "negative start", path: movieSubtitleStreamPath(subrip, "0", "?start=-3"), want: http.StatusBadRequest},
		{name: "malformed start", path: movieSubtitleStreamPath(subrip, "0", "?start=soon"), want: http.StatusBadRequest},
		{name: "unknown movie", path: movieSubtitleStreamPath(999999, "0", ""), want: http.StatusNotFound},
		{name: "malformed movie id", path: "/api/movies/abc/subtitles/0/stream.mkv", want: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := serveSubtitleStream(t, app, viewer.ID, tt.path)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.want, recorder.Body.String())
			}
			if !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("error answered as %q, want JSON", recorder.Header().Get("Content-Type"))
			}
		})
	}
	if len(fake.StreamCalls()) != 0 {
		t.Fatalf("FFmpeg ran for a rejected request: %+v", fake.StreamCalls())
	}

	anonymous := httptest.NewRecorder()
	app.Router.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, movieSubtitleStreamPath(subrip, "0", ""), nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", anonymous.Code)
	}
}

func TestEpisodeSubtitleStream_StreamsTheEpisodesFile(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{streamFunc: writingSubtitleStreams(testStreamHeader, nil)}
	app.FFmpeg = fake
	fixture := seedPlaybackEpisode(t, app)

	path := fmt.Sprintf("/api/shows/episodes/%d/subtitles/0/stream.mkv?start=12", fixture.Episode2)
	recorder := serveSubtitleStream(t, app, viewer.ID, path)

	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), testPrimedStream()) {
		t.Fatalf("response = %d %x", recorder.Code, recorder.Body.Bytes())
	}
	calls := fake.StreamCalls()
	want := ffmpeg.SubtitleStreamParams{InputPath: fixture.FilePath, StreamIndex: testSubtitleStreamIndex, StartSec: 12}
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("stream calls = %+v, want [%+v]", calls, want)
	}
}

func TestSubtitleStream_RefusesWhenEverySlotIsTaken(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, 20*time.Millisecond, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{streamFunc: writingSubtitleStreams(testStreamHeader, nil)}
	app.FFmpeg = fake
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	for range cap(app.SubtitleStreamSlots) {
		app.SubtitleStreamSlots <- struct{}{}
	}

	refused := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))
	if refused.Code != http.StatusServiceUnavailable || refused.Header().Get("Retry-After") != "2" {
		t.Fatalf("full = %d Retry-After %q: %s", refused.Code, refused.Header().Get("Retry-After"), refused.Body.String())
	}
	if len(fake.StreamCalls()) != 0 {
		t.Fatal("FFmpeg ran without a slot")
	}

	// A slot freed while the request waits is taken.
	subtitleStreamSlotWait = 5 * time.Second
	go func() {
		time.Sleep(50 * time.Millisecond)
		<-app.SubtitleStreamSlots
	}()
	admitted := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))
	if admitted.Code != http.StatusOK {
		t.Fatalf("after a slot freed = %d: %s", admitted.Code, admitted.Body.String())
	}
}

func TestSubtitleStream_RefusesOnceShutdownHasBegun(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{streamFunc: writingSubtitleStreams(testStreamHeader, nil)}
	app.FFmpeg = fake
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	app.beginHLSShutdown()

	recorder := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))

	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Retry-After") != "2" {
		t.Fatalf("during shutdown = %d Retry-After %q: %s", recorder.Code, recorder.Header().Get("Retry-After"), recorder.Body.String())
	}
	if len(fake.StreamCalls()) != 0 {
		t.Fatal("FFmpeg started during shutdown")
	}
}

func TestSubtitleStream_ReportsAStreamThatCannotBePrimed(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	tests := []struct {
		name    string
		output  []byte
		exitErr error
	}{
		{name: "FFmpeg ends before Tracks", output: testStreamHeader[:18], exitErr: errors.New("No such file or directory")},
		{name: "not Matroska", output: []byte("WEBVTT\n\n"), exitErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
			app.FFmpeg = &fakeFFmpeg{streamFunc: writingSubtitleStreams(tt.output, tt.exitErr)}
			movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")

			recorder := serveSubtitleStream(t, app, viewer.ID, movieSubtitleStreamPath(movieID, "0", ""))

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500: %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "failed to stream subtitle track") {
				t.Fatalf("body = %q, want the generic JSON error", recorder.Body.String())
			}
			if len(app.SubtitleStreamSlots) != 0 {
				t.Fatal("a failed start kept its stream slot")
			}
		})
	}
}

// readExactly reads n bytes of a streamed body, failing on a short read.
func readExactly(t *testing.T, body io.Reader, n int) []byte {
	t.Helper()
	out := make([]byte, n)
	_, err := io.ReadFull(body, out)
	if err != nil {
		t.Fatalf("read %d streamed bytes: %v", n, err)
	}
	return out
}

func waitForStreamEnd(t *testing.T, stream *scriptedSubtitleStream) {
	t.Helper()
	select {
	case <-stream.ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("FFmpeg was not stopped")
	}
}

func TestSubtitleStream_ClientLeavingStopsFFmpegAndFreesTheSlot(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	streams := make(chan *scriptedSubtitleStream, 1)
	app.FFmpeg = &fakeFFmpeg{streamFunc: heldSubtitleStreams(streams)}
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	server := subtitleStreamServer(t, app, viewer.ID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+movieSubtitleStreamPath(movieID, "0", ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	responses := make(chan *http.Response, 1)
	go func() {
		response, doErr := http.DefaultClient.Do(request)
		if doErr == nil {
			responses <- response
		}
	}()
	stream := receiveStream(t, streams)
	stream.write(t, testStreamHeader)
	response := <-responses
	defer response.Body.Close()
	primed := readExactly(t, response.Body, len(testPrimedStream()))
	if !bytes.Equal(primed, testPrimedStream()) {
		t.Fatalf("primed header = %x", primed)
	}

	cancel()
	waitForStreamEnd(t, stream)
	deadline := time.Now().Add(5 * time.Second)
	for len(app.SubtitleStreamSlots) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(app.SubtitleStreamSlots) != 0 {
		t.Fatal("the stream slot was not freed after the client left")
	}
}

func TestSubtitleStream_ShutdownEndsTheStreamCleanly(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	streams := make(chan *scriptedSubtitleStream, 1)
	app.FFmpeg = &fakeFFmpeg{streamFunc: heldSubtitleStreams(streams)}
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	server := subtitleStreamServer(t, app, viewer.ID)

	responses := make(chan *http.Response, 1)
	go func() {
		response, err := http.Get(server.URL + movieSubtitleStreamPath(movieID, "0", ""))
		if err == nil {
			responses <- response
		}
	}()
	stream := receiveStream(t, streams)
	stream.write(t, append(slices.Clone(testStreamHeader), testStreamCluster...))
	response := <-responses
	defer response.Body.Close()
	_ = readExactly(t, response.Body, len(testPrimedStream(testStreamCluster)))

	app.beginHLSShutdown()
	waitForStreamEnd(t, stream)
	rest, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("a stream ended by shutdown was cut: %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("bytes after shutdown: %x", rest)
	}
}

func TestSubtitleStream_FailureAfterTheHeaderCutsTheConnection(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	streams := make(chan *scriptedSubtitleStream, 1)
	app.FFmpeg = &fakeFFmpeg{streamFunc: heldSubtitleStreams(streams)}
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	server := subtitleStreamServer(t, app, viewer.ID)

	responses := make(chan *http.Response, 1)
	go func() {
		response, err := http.Get(server.URL + movieSubtitleStreamPath(movieID, "0", ""))
		if err == nil {
			responses <- response
		}
	}()
	stream := receiveStream(t, streams)
	stream.write(t, append(slices.Clone(testStreamHeader), testStreamCluster...))
	response := <-responses
	defer response.Body.Close()
	_ = readExactly(t, response.Body, len(testPrimedStream(testStreamCluster)))

	stream.write(t, testStreamCluster[:4])
	stream.end(errors.New("read error in the media file"))
	_, err := io.ReadAll(response.Body)
	if err == nil {
		t.Fatal("a stream FFmpeg failed partway through ended like a complete one")
	}
}

func TestSubtitleStream_CompleteStreamEndsCleanlyOverHTTP(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.FFmpeg = &fakeFFmpeg{streamFunc: writingSubtitleStreams(append(slices.Clone(testStreamHeader), testStreamCluster...), nil)}
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	server := subtitleStreamServer(t, app, viewer.ID)

	response, err := http.Get(server.URL + movieSubtitleStreamPath(movieID, "0", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("complete stream read: %v", err)
	}
	if !bytes.Equal(body, testPrimedStream(testStreamCluster)) {
		t.Fatalf("body = %x", body)
	}
	if response.ContentLength != -1 {
		t.Fatalf("Content-Length = %d, want a chunked stream", response.ContentLength)
	}
}

// A sparse track can leave FFmpeg silent for minutes while it reads ahead;
// Void elements between elements keep the connection, and the player's read,
// alive.
func TestSubtitleStream_KeepsASilentStreamAlive(t *testing.T) {
	overrideSubtitleStreamTiming(t, 20*time.Millisecond, time.Second, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	streams := make(chan *scriptedSubtitleStream, 1)
	app.FFmpeg = &fakeFFmpeg{streamFunc: heldSubtitleStreams(streams)}
	movieID, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	server := subtitleStreamServer(t, app, viewer.ID)

	responses := make(chan *http.Response, 1)
	go func() {
		response, err := http.Get(server.URL + movieSubtitleStreamPath(movieID, "0", ""))
		if err == nil {
			responses <- response
		}
	}()
	stream := receiveStream(t, streams)
	stream.write(t, testStreamHeader)
	response := <-responses
	defer response.Body.Close()
	_ = readExactly(t, response.Body, len(testPrimedStream()))

	keepalive := readExactly(t, response.Body, len(testStreamVoid))
	if !bytes.Equal(keepalive, testStreamVoid) {
		t.Fatalf("silence produced %x, want a Void element", keepalive)
	}
	stream.write(t, testStreamCluster)
	stream.end(nil)
	rest, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read rest: %v", err)
	}
	withoutVoids := bytes.ReplaceAll(rest, testStreamVoid, nil)
	if !bytes.Equal(withoutVoids, testStreamCluster) {
		t.Fatalf("rest = %x, want Voids then the cluster", rest)
	}
}

func TestSubtitleStreamResponsesConformToOpenAPI(t *testing.T) {
	overrideSubtitleStreamTiming(t, time.Hour, 10*time.Millisecond, time.Hour)
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.FFmpeg = &fakeFFmpeg{streamFunc: writingSubtitleStreams(append(slices.Clone(testStreamHeader), testStreamCluster...), nil)}
	subrip, _ := insertSubtitleStreamMovie(t, app, "mkv", "subrip")
	xsub, _ := insertSubtitleStreamMovie(t, app, "avi", "xsub")
	episode := seedPlaybackEpisode(t, app)
	handler := authenticatedRouter(t, app, viewer.ID)

	exchanges := []struct {
		operation string
		path      string
		status    int
	}{
		{"subtitleStream", movieSubtitleStreamPath(subrip, "0", "?start=12.5"), http.StatusOK},
		{"subtitleStream", movieSubtitleStreamPath(xsub, "0", ""), http.StatusUnsupportedMediaType},
		{"subtitleStream", movieSubtitleStreamPath(subrip, "4", ""), http.StatusBadRequest},
		{"subtitleStream", movieSubtitleStreamPath(999999, "0", ""), http.StatusNotFound},
		{"episodeSubtitleStream", fmt.Sprintf("/api/shows/episodes/%d/subtitles/0/stream.mkv", episode.Episode1), http.StatusOK},
		{"episodeSubtitleStream", "/api/shows/episodes/999999/subtitles/0/stream.mkv", http.StatusNotFound},
	}
	for _, exchange := range exchanges {
		serveOpenAPIExchange(t, handler, exchange.operation, httptest.NewRequest(http.MethodGet, exchange.path, nil), exchange.status)
	}

	for range cap(app.SubtitleStreamSlots) {
		app.SubtitleStreamSlots <- struct{}{}
	}
	serveOpenAPIExchange(t, handler, "subtitleStream", httptest.NewRequest(http.MethodGet, movieSubtitleStreamPath(subrip, "0", ""), nil), http.StatusServiceUnavailable)
}
