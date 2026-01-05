package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	applogger "igloo/cmd/internal/logger"

	"github.com/go-chi/chi/v5"
)

// testMediaFiles contains paths to actual audio files in the media directory.
// These are used to test real file streaming.
var testMediaFiles = []string{
	"track2.m4a",
	"track.m4a",
}

// getProjectRoot returns the root directory of the project.
// Works by walking up from the test file location.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current working directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up until we find the media directory
	for {
		if _, err := os.Stat(filepath.Join(wd, "media")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("Could not find project root with media directory")
		}
		wd = parent
	}
}

// setupTestAppWithLogger creates a test Application with an initialized database,
// tables, queries, and a logger for testing handlers.
func setupTestAppWithLogger(t *testing.T) *Application {
	t.Helper()

	app := setupTestApp(t)

	// Create a simple logger for testing (logs to stdout)
	logger, _, err := applogger.New(&applogger.LoggerConfig{
		Debug: true,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	app.Logger = logger

	return app
}

// insertTestTrack creates a track record in the database for testing.
func insertTestTrack(t *testing.T, app *Application, filePath, fileName, mimeType string) database.Track {
	t.Helper()

	// Get file info for size
	stat, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	track, err := app.Queries.UpsertTrack(context.Background(), database.UpsertTrackParams{
		Title:         fileName,
		SortTitle:     fileName,
		FilePath:      filePath,
		FileName:      fileName,
		Container:     "m4a",
		MimeType:      mimeType,
		Codec:         "aac",
		Size:          stat.Size(),
		TrackIndex:    1,
		Duration:      180000, // 3 minutes in ms
		Disc:          1,
		Channels:      "2",
		ChannelLayout: "stereo",
		BitRate:       256000,
		Profile:       "LC",
	})
	if err != nil {
		t.Fatalf("Failed to insert test track: %v", err)
	}

	return track
}

// TestGetTrackByID_Success tests successfully retrieving a track by its ID.
func TestGetTrackByID_Success(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	projectRoot := getProjectRoot(t)
	filePath := filepath.Join(projectRoot, "media", testMediaFiles[0])

	// Insert a test track
	track := insertTestTrack(t, app, filePath, testMediaFiles[0], "audio/mp4")

	// Create request with chi URL parameter
	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/details/1", nil)

	// Set up chi context with URL parameter
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.GetTrackByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify response body
	var response helpers.JSONResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Error {
		t.Error("Expected Error to be false")
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatal("Expected Data to be a map")
	}

	trackData, ok := data["track"].(map[string]any)
	if !ok {
		t.Fatal("Expected track data in response")
	}

	if trackData["title"] != track.Title {
		t.Errorf("Expected title '%s', got '%s'", track.Title, trackData["title"])
	}
}

// TestGetTrackByID_InvalidID tests that an invalid (non-numeric) ID returns a 400 error.
func TestGetTrackByID_InvalidID(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/details/invalid", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.GetTrackByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var response helpers.JSONResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Error {
		t.Error("Expected Error to be true")
	}

	if response.Message != "invalid track id" {
		t.Errorf("Expected message 'invalid track id', got '%s'", response.Message)
	}
}

// TestGetTrackByID_NotFound tests that a non-existent track ID returns a 404 error.
func TestGetTrackByID_NotFound(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/details/999", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.GetTrackByID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	var response helpers.JSONResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Error {
		t.Error("Expected Error to be true")
	}

	if response.Message != "track not found" {
		t.Errorf("Expected message 'track not found', got '%s'", response.Message)
	}
}

// TestStreamTrack_Success tests successfully streaming a track file.
func TestStreamTrack_Success(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	projectRoot := getProjectRoot(t)
	filePath := filepath.Join(projectRoot, "media", testMediaFiles[0])

	// Insert a test track
	insertTestTrack(t, app, filePath, testMediaFiles[0], "audio/mp4")

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/1/stream", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.StreamTrack(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify Content-Type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != "audio/mp4" {
		t.Errorf("Expected Content-Type 'audio/mp4', got '%s'", contentType)
	}

	// Verify Accept-Ranges header (should be set by http.ServeContent)
	acceptRanges := rr.Header().Get("Accept-Ranges")
	if acceptRanges != "bytes" {
		t.Errorf("Expected Accept-Ranges 'bytes', got '%s'", acceptRanges)
	}

	// Verify body has content
	if rr.Body.Len() == 0 {
		t.Error("Expected response body to have content")
	}
}

// TestStreamTrack_InvalidID tests that an invalid (non-numeric) ID returns a 400 error.
func TestStreamTrack_InvalidID(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/invalid/stream", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.StreamTrack(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var response helpers.JSONResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Error {
		t.Error("Expected Error to be true")
	}

	if response.Message != "invalid track id" {
		t.Errorf("Expected message 'invalid track id', got '%s'", response.Message)
	}
}

// TestStreamTrack_NotFound tests that a non-existent track ID returns a 404 error.
func TestStreamTrack_NotFound(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/999/stream", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.StreamTrack(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	var response helpers.JSONResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Error {
		t.Error("Expected Error to be true")
	}

	if response.Message != "track not found" {
		t.Errorf("Expected message 'track not found', got '%s'", response.Message)
	}
}

// TestStreamTrack_FileNotFound tests that a missing file on disk returns a 404 error.
func TestStreamTrack_FileNotFound(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	// Insert a track pointing to a non-existent file
	_, err := app.Queries.UpsertTrack(context.Background(), database.UpsertTrackParams{
		Title:         "Missing Track",
		SortTitle:     "missing track",
		FilePath:      "/path/to/nonexistent/file.m4a",
		FileName:      "file.m4a",
		Container:     "m4a",
		MimeType:      "audio/mp4",
		Codec:         "aac",
		Size:          1000,
		TrackIndex:    1,
		Duration:      180000,
		Disc:          1,
		Channels:      "2",
		ChannelLayout: "stereo",
		BitRate:       256000,
		Profile:       "LC",
	})
	if err != nil {
		t.Fatalf("Failed to insert test track: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/1/stream", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.StreamTrack(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	var response helpers.JSONResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Error {
		t.Error("Expected Error to be true")
	}

	if response.Message != "track file not found" {
		t.Errorf("Expected message 'track file not found', got '%s'", response.Message)
	}
}

// TestStreamTrack_RangeRequest tests HTTP Range requests for partial content (seeking).
func TestStreamTrack_RangeRequest(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	projectRoot := getProjectRoot(t)
	filePath := filepath.Join(projectRoot, "media", testMediaFiles[0])

	// Insert a test track
	track := insertTestTrack(t, app, filePath, testMediaFiles[0], "audio/mp4")

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/1/stream", nil)
	req.Header.Set("Range", "bytes=0-1023") // Request first 1KB

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.StreamTrack(rr, req)

	// Range requests should return 206 Partial Content
	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status %d, got %d", http.StatusPartialContent, rr.Code)
	}

	// Verify Content-Range header
	contentRange := rr.Header().Get("Content-Range")
	if contentRange == "" {
		t.Error("Expected Content-Range header to be set")
	}

	// Verify Content-Type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != track.MimeType {
		t.Errorf("Expected Content-Type '%s', got '%s'", track.MimeType, contentType)
	}

	// Verify we got exactly 1024 bytes (or less if file is smaller)
	if rr.Body.Len() > 1024 {
		t.Errorf("Expected at most 1024 bytes, got %d", rr.Body.Len())
	}
}

// TestStreamTrack_SecondMediaFile tests streaming the second media file.
func TestStreamTrack_SecondMediaFile(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	projectRoot := getProjectRoot(t)
	filePath := filepath.Join(projectRoot, "media", testMediaFiles[1])

	// Insert a test track
	insertTestTrack(t, app, filePath, testMediaFiles[1], "audio/mp4")

	req := httptest.NewRequest(http.MethodGet, "/api/music/tracks/1/stream", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.StreamTrack(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify Content-Type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != "audio/mp4" {
		t.Errorf("Expected Content-Type 'audio/mp4', got '%s'", contentType)
	}

	// Verify body has content
	if rr.Body.Len() == 0 {
		t.Error("Expected response body to have content")
	}
}

// TestStreamTrack_HeadRequest tests HEAD requests for the stream endpoint.
// HEAD requests should return headers without the body.
func TestStreamTrack_HeadRequest(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	projectRoot := getProjectRoot(t)
	filePath := filepath.Join(projectRoot, "media", testMediaFiles[0])

	// Insert a test track
	insertTestTrack(t, app, filePath, testMediaFiles[0], "audio/mp4")

	req := httptest.NewRequest(http.MethodHead, "/api/music/tracks/1/stream", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()

	app.StreamTrack(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify Content-Type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != "audio/mp4" {
		t.Errorf("Expected Content-Type 'audio/mp4', got '%s'", contentType)
	}

	// Verify Content-Length is set
	contentLength := rr.Header().Get("Content-Length")
	if contentLength == "" {
		t.Error("Expected Content-Length header to be set")
	}
}

// TestStreamTrack_MultipleRanges tests multiple consecutive range requests
// simulating audio player seeking behavior.
func TestStreamTrack_MultipleRanges(t *testing.T) {
	app := setupTestAppWithLogger(t)
	defer app.DB.Close()

	projectRoot := getProjectRoot(t)
	filePath := filepath.Join(projectRoot, "media", testMediaFiles[0])

	// Insert a test track
	insertTestTrack(t, app, filePath, testMediaFiles[0], "audio/mp4")

	// First range request - beginning of file
	req1 := httptest.NewRequest(http.MethodGet, "/api/music/tracks/1/stream", nil)
	req1.Header.Set("Range", "bytes=0-511")
	rctx1 := chi.NewRouteContext()
	rctx1.URLParams.Add("id", "1")
	req1 = req1.WithContext(context.WithValue(req1.Context(), chi.RouteCtxKey, rctx1))
	rr1 := httptest.NewRecorder()
	app.StreamTrack(rr1, req1)

	if rr1.Code != http.StatusPartialContent {
		t.Errorf("First range request: Expected status %d, got %d", http.StatusPartialContent, rr1.Code)
	}

	// Second range request - middle of file
	req2 := httptest.NewRequest(http.MethodGet, "/api/music/tracks/1/stream", nil)
	req2.Header.Set("Range", "bytes=512-1023")
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", "1")
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))
	rr2 := httptest.NewRecorder()
	app.StreamTrack(rr2, req2)

	if rr2.Code != http.StatusPartialContent {
		t.Errorf("Second range request: Expected status %d, got %d", http.StatusPartialContent, rr2.Code)
	}

	// Verify both requests returned different content ranges
	if rr1.Header().Get("Content-Range") == rr2.Header().Get("Content-Range") {
		t.Error("Expected different Content-Range headers for different byte ranges")
	}
}
