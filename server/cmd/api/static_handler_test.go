package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const testFrontendIndex = "<!doctype html><title>Igloo</title>"

func testFrontendFS() fstest.MapFS {
	return fstest.MapFS{
		"webdist/index.html":           {Data: []byte(testFrontendIndex)},
		"webdist/assets/app.abc123.js": {Data: []byte("console.log('igloo')")},
		"webdist/docs/index.html":      {Data: []byte("<!doctype html><title>Docs</title>")},
	}
}

func newFrontendTestApp(t *testing.T, fsys fs.FS) *Application {
	t.Helper()
	app := &Application{FrontendAssets: fsys}
	setupTestLogger(t, app)
	return app
}

func serveFrontend(app *Application, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	app.ServeFrontend(recorder, request)
	return recorder
}

func TestServeFrontend_ServesEmbeddedAssets(t *testing.T) {
	t.Setenv(envViteDevServer, "")
	app := newFrontendTestApp(t, testFrontendFS())

	tests := []struct {
		name             string
		target           string
		wantStatus       int
		wantBody         string
		wantCacheControl string
		wantContentType  string
	}{
		{"root serves index", "/", http.StatusOK, testFrontendIndex, "no-cache, no-store, must-revalidate", "text/html; charset=utf-8"},
		{"hashed asset is immutable", "/assets/app.abc123.js", http.StatusOK, "console.log('igloo')", "public, max-age=31536000", "text/javascript; charset=utf-8"},
		{"directory serves its index", "/docs", http.StatusOK, "<!doctype html><title>Docs</title>", "no-cache, no-store, must-revalidate", "text/html; charset=utf-8"},
		{"client route falls back to index", "/movies/42", http.StatusOK, testFrontendIndex, "no-cache, no-store, must-revalidate", "text/html; charset=utf-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := serveFrontend(app, http.MethodGet, tt.target, nil)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if recorder.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), tt.wantBody)
			}
			if got := recorder.Header().Get("Cache-Control"); got != tt.wantCacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCacheControl)
			}
			if got := recorder.Header().Get("Content-Type"); got != tt.wantContentType {
				t.Errorf("Content-Type = %q, want %q", got, tt.wantContentType)
			}
			if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if recorder.Header().Get("ETag") == "" {
				t.Error("ETag was not set")
			}
		})
	}
}

func TestServeFrontend_ETagRevalidation(t *testing.T) {
	t.Setenv(envViteDevServer, "")
	app := newFrontendTestApp(t, testFrontendFS())

	first := serveFrontend(app, http.MethodGet, "/", nil)
	etag := first.Header().Get("ETag")
	if first.Code != http.StatusOK || etag == "" {
		t.Fatalf("first response = %d, ETag %q", first.Code, etag)
	}

	revalidated := serveFrontend(app, http.MethodGet, "/", map[string]string{"If-None-Match": etag})
	if revalidated.Code != http.StatusNotModified {
		t.Fatalf("status with matching ETag = %d, want 304", revalidated.Code)
	}
	if revalidated.Body.Len() != 0 {
		t.Fatalf("304 carried a body: %q", revalidated.Body.String())
	}

	changed := serveFrontend(app, http.MethodGet, "/", map[string]string{"If-None-Match": `"stale"`})
	if changed.Code != http.StatusOK {
		t.Fatalf("status with stale ETag = %d, want 200", changed.Code)
	}
}

func TestServeFrontend_RejectionsAndMissingAssets(t *testing.T) {
	t.Setenv(envViteDevServer, "")

	tests := []struct {
		name       string
		fsys       fs.FS
		target     string
		wantStatus int
		wantBody   string
	}{
		{"path traversal is forbidden", testFrontendFS(), "/../etc/passwd", http.StatusForbidden, "Forbidden"},
		{"missing hashed asset does not fall back to index", testFrontendFS(), "/assets/missing.js", http.StatusNotFound, "embedded static asset missing"},
		{"missing bundle reports the build step", fstest.MapFS{}, "/", http.StatusNotFound, "Frontend not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newFrontendTestApp(t, tt.fsys)
			recorder := serveFrontend(app, http.MethodGet, tt.target, nil)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want it to mention %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestServeFrontend_RedirectsToViteDevServer(t *testing.T) {
	t.Setenv(envViteDevServer, "http://localhost:3000/")
	app := newFrontendTestApp(t, testFrontendFS())

	recorder := serveFrontend(app, http.MethodGet, "/movies/42?tab=cast", nil)
	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Location"); got != "http://localhost:3000/movies/42?tab=cast" {
		t.Fatalf("Location = %q, want the Vite URL with the path and query preserved", got)
	}
}

// HEAD has its own registration in routes.go because chi answers HEAD on a
// GET-only route with 405; the SPA must answer it with headers and no body.
func TestServeFrontend_HeadThroughRouter(t *testing.T) {
	t.Setenv(envViteDevServer, "")
	app := setupTestApp(t)
	app.FrontendAssets = testFrontendFS()
	handler := authenticatedRouter(t, app, 0)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodHead, "/movies/42", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("HEAD carried a body: %q", recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
}

func TestEmbeddedPathLooksLikeStaticAsset(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"assets/app.abc123.js", true},
		{"assets/app.abc123.css", true},
		{"assets/app.abc123.js.map", true},
		{"fonts/inter.woff2", true},
		{"favicon.ico", true},
		{"manifest.webmanifest", true},
		{"ASSETS/APP.JS", true},
		{"movies/42", false},
		{"index.html", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := embeddedPathLooksLikeStaticAsset(tt.path); got != tt.want {
			t.Errorf("embeddedPathLooksLikeStaticAsset(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// The static-file contract documents text/plain errors, so the guards are
// asserted on status and body rather than the JSON envelope.
func TestServeStaticFiles_RejectsTraversalDirectoriesAndMissingFiles(t *testing.T) {
	app := setupTestApp(t)
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	handler := authenticatedRouter(t, app, viewer.ID)

	staticDir := app.CurrentSettings().StaticDir
	err := os.MkdirAll(filepath.Join(staticDir, "posters"), 0o755)
	if err != nil {
		t.Fatalf("create static subdirectory: %v", err)
	}
	err = os.WriteFile(filepath.Join(staticDir, "posters", "one.png"), []byte("png"), 0o644)
	if err != nil {
		t.Fatalf("write static file: %v", err)
	}

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantBody   string
	}{
		{"served file", "/api/static/posters/one.png", http.StatusOK, "png"},
		{"parent traversal", "/api/static/../igloo.db", http.StatusForbidden, "Forbidden"},
		{"directory", "/api/static/posters", http.StatusForbidden, "Forbidden"},
		{"missing file", "/api/static/posters/two.png", http.StatusNotFound, "Not Found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.target, nil))
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want it to contain %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}
