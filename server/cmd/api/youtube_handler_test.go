package main

import (
	"encoding/json"
	"igloo/cmd/internal/helpers"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestProxyYouTubeThumbnail_HTTPSuccessStreamsImage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dQw4w9WgXcQ/hqdefault.jpg" {
			t.Fatalf("upstream path = %q, want /dQw4w9WgXcQ/hqdefault.jpg", r.URL.Path)
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{1, 2, 3, 4})
	}))
	defer upstream.Close()

	app := setupTestApp(t)
	defer app.DB.Close()
	app.YouTubeThumbBaseURL = upstream.URL
	app.YouTubeThumbHTTPClient = upstream.Client()

	router := chi.NewRouter()
	router.Get("/api/youtube/thumbnails/{key}", app.ProxyYouTubeThumbnail)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/youtube/thumbnails/dQw4w9WgXcQ", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Fatalf("cache control = %q, want upstream cache header", got)
	}
	if got := w.Body.Bytes(); string(got) != string([]byte{1, 2, 3, 4}) {
		t.Fatalf("body = %#v, want image bytes", got)
	}
}

func TestProxyYouTubeThumbnail_HTTPRejectsInvalidKey(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	router := chi.NewRouter()
	router.Get("/api/youtube/thumbnails/{key}", app.ProxyYouTubeThumbnail)

	tests := []string{
		"/api/youtube/thumbnails/bad$key",
		"/api/youtube/thumbnails/bad.key",
		"/api/youtube/thumbnails/" + strings.Repeat("a", youtubeVideoKeyMaxLength+1),
	}

	for _, path := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400, body = %s", path, w.Code, w.Body.String())
		}

		var resp helpers.JSONResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode invalid response: %v", err)
		}
		if !resp.Error {
			t.Fatalf("%s response = %+v, want error", path, resp)
		}
	}
}

func TestProxyYouTubeThumbnail_HTTPReturnsErrorForUpstreamFailures(t *testing.T) {
	t.Run("non-200", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "missing", http.StatusNotFound)
		}))
		defer upstream.Close()

		app := setupTestApp(t)
		defer app.DB.Close()
		app.YouTubeThumbBaseURL = upstream.URL
		app.YouTubeThumbHTTPClient = upstream.Client()

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/youtube/thumbnails/dQw4w9WgXcQ", nil)
		router := chi.NewRouter()
		router.Get("/api/youtube/thumbnails/{key}", app.ProxyYouTubeThumbnail)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502, body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		app := setupTestApp(t)
		defer app.DB.Close()
		app.YouTubeThumbBaseURL = "https://i.ytimg.com/vi"
		app.YouTubeThumbHTTPClient = &http.Client{Transport: failingRoundTripper{}}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/youtube/thumbnails/dQw4w9WgXcQ", nil)
		router := chi.NewRouter()
		router.Get("/api/youtube/thumbnails/{key}", app.ProxyYouTubeThumbnail)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502, body = %s", w.Code, w.Body.String())
		}

		var resp helpers.JSONResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode fetch error response: %v", err)
		}
		if !resp.Error {
			t.Fatalf("response = %+v, want error", resp)
		}
	})
}

func TestIsSafeYouTubeVideoKey(t *testing.T) {
	valid := []string{"dQw4w9WgXcQ", "a", "abc_DEF-123"}
	for _, key := range valid {
		if !isSafeYouTubeVideoKey(key) {
			t.Fatalf("isSafeYouTubeVideoKey(%q) = false, want true", key)
		}
	}

	invalid := []string{
		"",
		"bad key",
		"bad/key",
		"bad.key",
		"bad$key",
		"..",
		strings.Repeat("a", youtubeVideoKeyMaxLength+1),
	}
	for _, key := range invalid {
		if isSafeYouTubeVideoKey(key) {
			t.Fatalf("isSafeYouTubeVideoKey(%q) = true, want false", key)
		}
	}
}
