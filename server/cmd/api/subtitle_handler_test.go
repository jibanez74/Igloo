package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func insertTestSubtitleFixture(t *testing.T, app *Application, movieID int64, streamIndex int64, codec string) {
	t.Helper()

	_, err := app.DB.Exec(`
		INSERT INTO subtitles (movie_id, stream_index, codec, language)
		VALUES (?, ?, ?, ?)
	`, movieID, streamIndex, codec, "eng")
	if err != nil {
		t.Fatalf("insert subtitle: %v", err)
	}
}

func serveSubtitleWebVTT(app *Application, movieID int64, trackIndex string) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	router.Get("/api/movies/{id}/subtitles/{trackIndex}/web.vtt", app.SubtitleWebVTT)

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/movies/%d/subtitles/%s/web.vtt", movieID, trackIndex),
		nil,
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func serveSubtitleWebVTTWithQuery(
	app *Application,
	movieID int64,
	trackIndex string,
	query string,
) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	router.Get("/api/movies/{id}/subtitles/{trackIndex}/web.vtt", app.SubtitleWebVTT)

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
	defer app.DB.Close()
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitleFixture(t, app, movieID, 2, "subrip")

	absolute := "WEBVTT\n\n00:10:05.000 --> 00:10:07.000\nLine\n"
	app.SubtitleVTTCache.Set(helpers.SubtitleCacheKey(movieID, 2), []byte(absolute), subtitleCacheTTL)

	recorder := serveSubtitleWebVTTWithQuery(app, movieID, "0", "?start=600")

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "00:00:05.000 --> 00:00:07.000") {
		t.Fatalf("cues were not rebased onto the session: %s", recorder.Body.String())
	}

	// The cache holds the absolute payload, so an unshifted request still gets
	// absolute cues without a second extraction.
	unshifted := serveSubtitleWebVTT(app, movieID, "0")
	if !strings.Contains(unshifted.Body.String(), "00:10:05.000 --> 00:10:07.000") {
		t.Fatalf("cache must keep absolute cues: %s", unshifted.Body.String())
	}
	if fake.SubtitleCallCount() != 0 {
		t.Fatalf("shifting must not trigger extraction, got %d calls", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_RejectsNegativeStart(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitleFixture(t, app, movieID, 2, "subrip")

	recorder := serveSubtitleWebVTTWithQuery(app, movieID, "0", "?start=-5")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a negative start, got %d", recorder.Code)
	}
}

func TestSubtitleWebVTT_ExtractsTextSubtitle(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitleFixture(t, app, movieID, 2, "subrip")

	recorder := serveSubtitleWebVTT(app, movieID, "0")

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
}

func TestSubtitleWebVTT_SecondRequestServedFromCache(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitleFixture(t, app, movieID, 2, "subrip")

	first := serveSubtitleWebVTT(app, movieID, "0")
	if first.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", first.Code)
	}

	second := serveSubtitleWebVTT(app, movieID, "0")
	if second.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", second.Code)
	}
	if second.Body.String() != "WEBVTT\n" {
		t.Fatalf("second body = %q, want cached extractor output", second.Body.String())
	}
	if fake.SubtitleCallCount() != 1 {
		t.Fatalf("extractor call count = %d, want 1 (second request must hit the cache)", fake.SubtitleCallCount())
	}
}

func TestSubtitleWebVTT_RejectsBitmapSubtitle(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	fake := &fakeFFmpeg{}
	app.FFmpeg = fake

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitleFixture(t, app, movieID, 2, "hdmv_pgs_subtitle")

	recorder := serveSubtitleWebVTT(app, movieID, "0")

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
	defer app.DB.Close()
	app.FFmpeg = &fakeFFmpeg{}

	movieID := insertTestHLSMovieFixture(t, app, "h264", 1080)
	insertTestSubtitleFixture(t, app, movieID, 2, "subrip")

	recorder := serveSubtitleWebVTT(app, movieID, "5")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
