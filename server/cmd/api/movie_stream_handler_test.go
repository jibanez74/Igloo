package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"

	"github.com/go-chi/chi/v5"
)

func seedStreamTestMovie(t *testing.T, app *Application, container, mimeType string, content []byte) database.Movie {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "Stream Test (2024)."+container)
	err := os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatalf("write movie file: %v", err)
	}

	movie, err := app.Queries.UpsertMovie(context.Background(), database.UpsertMovieParams{
		Title:     "Stream Test",
		FilePath:  path,
		FileName:  filepath.Base(path),
		Size:      int64(len(content)),
		Container: container,
		MimeType:  mimeType,
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	return movie
}

func streamTestHandler(app *Application) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/movies/{id}/stream", app.StreamMovie)
	r.Head("/api/movies/{id}/stream", app.StreamMovie)
	return r
}

func TestStreamMovieServesFullFileWithPinnedContentType(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(app)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "video/mp4" {
		t.Errorf("Content-Type = %q, want video/mp4", got)
	}
	if got := w.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q, want bytes", got)
	}
	if got := w.Header().Get("Content-Length"); got != fmt.Sprintf("%d", len(content)) {
		t.Errorf("Content-Length = %q, want %d", got, len(content))
	}
	if !bytes.Equal(w.Body.Bytes(), content) {
		t.Error("body does not match file content")
	}
}

func TestStreamMovieDerivesContentTypeFromContainer(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	// A pre-fix row scanned on a host without /etc/mime.types stored
	// application/octet-stream; the handler must still answer from the
	// pinned container map.
	movie := seedStreamTestMovie(t, app, "mkv", "application/octet-stream", []byte("matroska"))
	handler := streamTestHandler(app)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "video/x-matroska" {
		t.Errorf("Content-Type = %q, want video/x-matroska", got)
	}
}

func TestStreamMovieServesRanges(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(app)
	size := len(content)

	cases := []struct {
		name        string
		rangeHeader string
		wantStatus  int
		wantRange   string
		wantBody    []byte
	}{
		{
			name:        "bounded range",
			rangeHeader: "bytes=0-99",
			wantStatus:  http.StatusPartialContent,
			wantRange:   fmt.Sprintf("bytes 0-99/%d", size),
			wantBody:    content[0:100],
		},
		{
			name:        "open-ended range",
			rangeHeader: fmt.Sprintf("bytes=%d-", size-50),
			wantStatus:  http.StatusPartialContent,
			wantRange:   fmt.Sprintf("bytes %d-%d/%d", size-50, size-1, size),
			wantBody:    content[size-50:],
		},
		{
			name:        "suffix range",
			rangeHeader: "bytes=-100",
			wantStatus:  http.StatusPartialContent,
			wantRange:   fmt.Sprintf("bytes %d-%d/%d", size-100, size-1, size),
			wantBody:    content[size-100:],
		},
		{
			name:        "unsatisfiable range",
			rangeHeader: fmt.Sprintf("bytes=%d-", size+100),
			wantStatus:  http.StatusRequestedRangeNotSatisfiable,
			wantRange:   fmt.Sprintf("bytes */%d", size),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
			req.Header.Set("Range", tc.rangeHeader)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if got := w.Header().Get("Content-Range"); got != tc.wantRange {
				t.Errorf("Content-Range = %q, want %q", got, tc.wantRange)
			}
			if tc.wantBody != nil && !bytes.Equal(w.Body.Bytes(), tc.wantBody) {
				t.Errorf("body = %d bytes, want the exact %d requested bytes", w.Body.Len(), len(tc.wantBody))
			}
		})
	}
}

func TestStreamMovieHead(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(app)

	t.Run("plain head", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if got := w.Header().Get("Content-Length"); got != fmt.Sprintf("%d", len(content)) {
			t.Errorf("Content-Length = %q, want %d", got, len(content))
		}
		if got := w.Header().Get("Accept-Ranges"); got != "bytes" {
			t.Errorf("Accept-Ranges = %q, want bytes", got)
		}
		if got := w.Header().Get("Content-Type"); got != "video/mp4" {
			t.Errorf("Content-Type = %q, want video/mp4", got)
		}
		if w.Body.Len() != 0 {
			t.Errorf("body = %d bytes, want empty", w.Body.Len())
		}
	})

	t.Run("head with range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
		req.Header.Set("Range", "bytes=0-99")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusPartialContent)
		}
		if got := w.Header().Get("Content-Range"); got != fmt.Sprintf("bytes 0-99/%d", len(content)) {
			t.Errorf("Content-Range = %q, want bytes 0-99/%d", got, len(content))
		}
		if w.Body.Len() != 0 {
			t.Errorf("body = %d bytes, want empty", w.Body.Len())
		}
	})
}

// TestStreamMovieOverSessionMiddleware exercises the delivery path a browser
// actually takes. httptest.ResponseRecorder does not implement io.ReaderFrom,
// so the tests above never reach the zero-copy branch that restoreSendfile
// enables; this one runs over a real socket behind the same middleware pair
// InitRouter installs.
func TestStreamMovieOverSessionMiddleware(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	content := bytes.Repeat([]byte("0123456789"), 4096)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)

	router := chi.NewRouter()
	router.Use(app.LoadAndSaveSession)
	router.Use(restoreSendfile)
	router.Get("/api/movies/{id}/stream", app.StreamMovie)

	server := httptest.NewServer(router)
	defer server.Close()

	streamURL := fmt.Sprintf("%s/api/movies/%d/stream", server.URL, movie.ID)

	t.Run("full body", func(t *testing.T) {
		res, err := http.Get(streamURL)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, content) {
			t.Errorf("body = %d bytes, want %d matching bytes", len(body), len(content))
		}
	})

	t.Run("range", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, streamURL, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Range", "bytes=10-19")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusPartialContent {
			t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusPartialContent)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, content[10:20]) {
			t.Errorf("body = %q, want %q", body, content[10:20])
		}
	})
}

func TestStreamMovieETagValidation(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(app)
	target := fmt.Sprintf("/api/movies/%d/stream", movie.ID)

	fetchETag := func(t *testing.T, method string) string {
		t.Helper()
		req := httptest.NewRequest(method, target, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		etag := w.Header().Get("ETag")
		if etag == "" {
			t.Fatalf("%s response has no ETag", method)
		}
		return etag
	}

	etag := fetchETag(t, http.MethodGet)
	stat, err := os.Stat(movie.FilePath)
	if err != nil {
		t.Fatalf("stat movie file: %v", err)
	}
	want := fmt.Sprintf("\"%x-%x\"", stat.Size(), stat.ModTime().UnixNano())
	if etag != want {
		t.Errorf("ETag = %q, want %q", etag, want)
	}
	if headETag := fetchETag(t, http.MethodHead); headETag != etag {
		t.Errorf("HEAD ETag = %q, want %q", headETag, etag)
	}

	t.Run("if-none-match", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotModified {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotModified)
		}
		if w.Body.Len() != 0 {
			t.Errorf("body = %d bytes, want empty", w.Body.Len())
		}
	})

	t.Run("if-range match serves the range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Range", "bytes=0-99")
		req.Header.Set("If-Range", etag)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusPartialContent {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusPartialContent)
		}
		if !bytes.Equal(w.Body.Bytes(), content[0:100]) {
			t.Errorf("body = %d bytes, want the first 100 bytes", w.Body.Len())
		}
	})

	t.Run("if-range mismatch serves the full file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Range", "bytes=0-99")
		req.Header.Set("If-Range", "\"stale-etag\"")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if !bytes.Equal(w.Body.Bytes(), content) {
			t.Errorf("body = %d bytes, want the full %d bytes", w.Body.Len(), len(content))
		}
	})
}

func TestStreamMovieErrorPaths(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", []byte("gone soon"))
	err := os.Remove(movie.FilePath)
	if err != nil {
		t.Fatalf("remove movie file: %v", err)
	}
	handler := streamTestHandler(app)

	cases := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{name: "invalid id", target: "/api/movies/not-a-number/stream", wantStatus: http.StatusBadRequest},
		{name: "missing row", target: "/api/movies/999999/stream", wantStatus: http.StatusNotFound},
		{name: "file gone", target: fmt.Sprintf("/api/movies/%d/stream", movie.ID), wantStatus: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
		})
	}
}
