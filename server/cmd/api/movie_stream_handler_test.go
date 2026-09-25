package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
)

func seedStreamTestMovie(t *testing.T, app *Application, container, mimeType string, content []byte) database.Movie {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "Stream Test (2024)."+container)
	err := os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatalf("write movie file: %v", err)
	}

	movieID, err := app.Queries.UpsertMovie(context.Background(), database.UpsertMovieParams{
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
	movie, err := app.Queries.GetMovieByID(context.Background(), movieID)
	if err != nil {
		t.Fatal(err)
	}
	return movie
}

func streamTestHandler(t *testing.T, app *Application) http.Handler {
	t.Helper()
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	return authenticatedRouter(t, app, viewer.ID)
}

func TestStreamMovieDerivesContentTypeFromContainer(t *testing.T) {
	app := setupTestApp(t)

	// A pre-fix row scanned on a host without /etc/mime.types stored
	// application/octet-stream; the handler must still answer from the
	// pinned container map.
	movie := seedStreamTestMovie(t, app, "mkv", "application/octet-stream", []byte("matroska"))
	handler := streamTestHandler(t, app)

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

// The bounded, conditional and unsatisfiable range forms are exercised for
// every media route by TestMediaResponsesConformToOpenAPI; the open-ended and
// suffix forms are only sent here.
func TestStreamMovieServesRanges(t *testing.T) {
	app := setupTestApp(t)

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(t, app)
	size := len(content)

	cases := []struct {
		name        string
		rangeHeader string
		wantStatus  int
		wantRange   string
		wantBody    []byte
	}{
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

// TestMediaResponsesConformToOpenAPI proves an ETag is present and honoured;
// this pins the format the validator is derived from.
func TestStreamMovieETagFormat(t *testing.T) {
	app := setupTestApp(t)

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(t, app)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	stat, err := os.Stat(movie.FilePath)
	if err != nil {
		t.Fatalf("stat movie file: %v", err)
	}
	want := fmt.Sprintf("\"%x-%x\"", stat.Size(), stat.ModTime().UnixNano())
	if etag := w.Header().Get("ETag"); etag != want {
		t.Errorf("ETag = %q, want %q", etag, want)
	}
}

func TestStreamMovieErrorPaths(t *testing.T) {
	app := setupTestApp(t)

	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", []byte("gone soon"))
	err := os.Remove(movie.FilePath)
	if err != nil {
		t.Fatalf("remove movie file: %v", err)
	}
	handler := streamTestHandler(t, app)

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
