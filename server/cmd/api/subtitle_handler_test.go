package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
