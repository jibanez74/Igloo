package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

// The subtitle fixtures all live on stream 2, matching a typical container
// where the subtitle track follows one video and one audio stream.
const testSubtitleStreamIndex = 2

func serveSubtitleWebVTTWithQuery(
	t *testing.T,
	app *Application,
	userID int64,
	movieID int64,
	trackIndex string,
	query string,
) *httptest.ResponseRecorder {
	t.Helper()
	router := authenticatedRouter(t, app, userID)

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/%d/subtitles/%s/web.vtt%s", movieID, trackIndex, query),
		nil,
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// A rebased HLS session's media timeline starts at zero while these cues carry
// absolute source timestamps, so `start` rebases them onto the session they
// will be played against (audit H4).
func TestSubtitleWebVTT_ShiftsCuesBySessionStart(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	absolute := "WEBVTT\n\n00:10:05.000 --> 00:10:07.000\nLine\n"
	app.SubtitleVTTCache.Set(helpers.SubtitleCacheKey("movie", movieID, 2), []byte(absolute), subtitleCacheTTL)

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "?start=600")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "00:00:05.000 --> 00:00:07.000") {
		t.Fatalf("cues were not rebased onto the session: %s", recorder.Body.String())
	}

	// The cache holds the absolute payload, so an unshifted request still gets
	// absolute cues without a second extraction.
	unshifted := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if !strings.Contains(unshifted.Body.String(), "00:10:05.000 --> 00:10:07.000") {
		t.Fatalf("cache must keep absolute cues: %s", unshifted.Body.String())
	}
	if fake.SubtitleCallCount() != 0 {
		t.Fatalf("shifting must not trigger extraction, got %d calls", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_RejectsNegativeStart(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "?start=-5")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a negative start, got %d", recorder.Code)
	}
}

func TestSubtitleWebVTT_ExtractsOnceAndServesFromCache(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != subtitleWebVTTContentType {
		t.Fatalf("Content-Type = %q, want %q", got, subtitleWebVTTContentType)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	if recorder.Body.String() != "WEBVTT\n" {
		t.Fatalf("body = %q, want extractor output", recorder.Body.String())
	}
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count = %d, want 1", fake.SubtitleCallCount())
	}

	second := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if second.Code != http.StatusOK || second.Body.String() != "WEBVTT\n" {
		t.Fatalf("second request = %d %q, want the cached extractor output", second.Code, second.Body.String())
	}
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count = %d, want 1 (second request must hit the cache)", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_RejectsBitmapSubtitle(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "hdmv_pgs_subtitle", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var envelope struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not the JSON error envelope: %v", err)
	}
	if !envelope.Error || envelope.Message == "" {
		t.Fatalf("envelope = %+v, want error=true with a message", envelope)
	}
	if fake.SubtitleCallCount() != 0 {
		t.Fatalf("extractor call count = %d, want 0 for bitmap codec", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_TrackIndexOutOfRange(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "5", "")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// blockingSubtitleFFmpeg holds every extraction until release closes, and
// records whether the extraction's context had been canceled by then.
type blockingSubtitleFFmpeg struct {
	*fakeFFmpeg
	started  chan struct{}
	release  chan struct{}
	canceled atomic.Bool
}

func (f *blockingSubtitleFFmpeg) ExtractSubtitleAsWebVTT(ctx context.Context, path string, streamIndex int64) ([]byte, error) {
	close(f.started)
	<-f.release
	if ctx.Err() != nil {
		f.canceled.Store(true)
		return nil, ctx.Err()
	}
	return f.fakeFFmpeg.ExtractSubtitleAsWebVTT(ctx, path, streamIndex)
}

// A player drops its subtitle request whenever it rebuilds the track, which
// every rebase does. The extraction that request started is shared, so it has
// to finish and be cached rather than die with the request.
func TestSubtitleWebVTT_ExtractionOutlivesTheRequestThatStartedIt(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &blockingSubtitleFFmpeg{
		fakeFFmpeg: &fakeFFmpeg{},
		started:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	router := authenticatedRouter(t, app, viewer.ID)
	ctx, cancel := context.WithCancel(context.Background())
	first := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/subtitles/0/web.vtt", movieID), nil).WithContext(ctx)
	firstRecorder := httptest.NewRecorder()
	served := make(chan struct{})
	go func() {
		router.ServeHTTP(firstRecorder, first)
		close(served)
	}()

	select {
	case <-fake.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the extraction never started")
	}
	cancel()
	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the canceled request kept waiting on the extraction")
	}
	if firstRecorder.Body.Len() != 0 {
		t.Fatalf("a request whose client left was answered: %d %q", firstRecorder.Code, firstRecorder.Body.String())
	}

	close(fake.release)
	app.Wait.Wait()
	if fake.canceled.Load() {
		t.Fatal("the shared extraction was canceled with the request that started it")
	}

	second := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if second.Code != http.StatusOK || second.Body.String() != "WEBVTT\n" {
		t.Fatalf("next request = %d %q, want the cached extraction", second.Code, second.Body.String())
	}
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count = %d, want 1", fake.SubtitleCallCount())
	}
}

// Without a refresh, a track extracted at the start of a long film expires
// mid-watch, and the next rebase pays for a whole extraction again.
func TestSubtitleWebVTT_CacheHitRestartsTheTTL(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitle(t, app, movieID, testSubtitleStreamIndex, "subrip", "eng")

	key := helpers.SubtitleCacheKey("movie", movieID, testSubtitleStreamIndex)
	app.SubtitleVTTCache.Set(key, []byte("WEBVTT\n"), time.Minute)

	recorder := serveSubtitleWebVTTWithQuery(t, app, viewer.ID, movieID, "0", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	_, expiration, found := app.SubtitleVTTCache.GetWithExpiration(key)
	if !found || time.Until(expiration) < subtitleCacheTTL-time.Minute {
		t.Fatalf("cache entry expires in %s after a hit, want about %s", time.Until(expiration), subtitleCacheTTL)
	}
	if fake.SubtitleCallCount() != 0 {
		t.Fatalf("extractor call count = %d, want 0 for a cache hit", fake.SubtitleCallCount())
	}
}
