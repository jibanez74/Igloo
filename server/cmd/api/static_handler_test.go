package main

import (
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
		"webdist/manifest.webmanifest": {Data: []byte(`{"name":"Igloo"}`)},
		"webdist/robots.txt":           {Data: []byte("User-agent: *\nDisallow: /\n")},
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
		{"hashed asset is immutable", "/assets/app.abc123.js", http.StatusOK, "console.log('igloo')", "public, max-age=31536000, immutable", "text/javascript; charset=utf-8"},
		{"directory serves its index", "/docs", http.StatusOK, "<!doctype html><title>Docs</title>", "no-cache, no-store, must-revalidate", "text/html; charset=utf-8"},
		{"client route falls back to index", "/movies/42", http.StatusOK, testFrontendIndex, "no-cache, no-store, must-revalidate", "text/html; charset=utf-8"},
		{"manifest revalidates with its media type", "/manifest.webmanifest", http.StatusOK, `{"name":"Igloo"}`, "no-cache", "application/manifest+json"},
		{"robots.txt is served, not the SPA", "/robots.txt", http.StatusOK, "User-agent: *\nDisallow: /\n", "no-cache", "text/plain; charset=utf-8"},
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
			if got := recorder.Header().Get("X-Robots-Tag"); got != "noindex, nofollow" {
				t.Errorf("X-Robots-Tag = %q, want noindex, nofollow", got)
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

// The SPA is served outside the session middleware: its files are the same for
// everyone, and the session's Vary: Cookie had browsers revalidate year-cached
// assets whenever the cookie changed. The API keeps its session.
func TestServeFrontend_SkipsTheSessionThroughRouter(t *testing.T) {
	t.Setenv(envViteDevServer, "")
	app := setupTestApp(t)
	app.FrontendAssets = testFrontendFS()
	handler := authenticatedRouter(t, app, 1)

	for _, target := range []string{"/", "/assets/app.abc123.js", "/movies/42"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200: %s", target, recorder.Code, recorder.Body.String())
		}
		for _, vary := range recorder.Header().Values("Vary") {
			if strings.Contains(vary, "Cookie") {
				t.Fatalf("%s varies on %q, want the SPA kept out of the session", target, vary)
			}
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/user", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("API status = %d, want 200 for the session's user: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(strings.Join(recorder.Header().Values("Vary"), ","), "Cookie") {
		t.Fatalf("API Vary = %q, want the session middleware's Cookie", recorder.Header().Values("Vary"))
	}
}

// Text assets are compressed once at load and sent gzipped to clients that ask
// for it; everyone else, and anything gzip barely shrinks, gets the original.
func TestServeFrontend_GzipsTextAssetsForClientsThatAcceptIt(t *testing.T) {
	t.Setenv(envViteDevServer, "")
	bundle := strings.Repeat("console.log('igloo');\n", 500)
	fsys := testFrontendFS()
	fsys["webdist/assets/bundle.abc123.js"] = &fstest.MapFile{Data: []byte(bundle)}
	fsys["webdist/fonts/inter.woff2"] = &fstest.MapFile{Data: []byte(bundle)}
	app := newFrontendTestApp(t, fsys)

	gzipped := serveFrontend(app, http.MethodGet, "/assets/bundle.abc123.js", map[string]string{"Accept-Encoding": "br, gzip, deflate"})
	if gzipped.Code != http.StatusOK || gzipped.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip request = %d with Content-Encoding %q, want a gzipped 200", gzipped.Code, gzipped.Header().Get("Content-Encoding"))
	}
	if gzipped.Header().Get("Content-Length") != strconv.Itoa(gzipped.Body.Len()) || gzipped.Body.Len() >= len(bundle) {
		t.Fatalf("gzipped body is %d bytes with Content-Length %q, want a smaller body of that length", gzipped.Body.Len(), gzipped.Header().Get("Content-Length"))
	}
	reader, err := gzip.NewReader(gzipped.Body)
	if err != nil {
		t.Fatalf("open gzipped body: %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || string(decoded) != bundle {
		t.Fatalf("gzipped body decodes to %d bytes (err %v), want the original %d", len(decoded), err, len(bundle))
	}
	if gzipped.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("Vary = %q, want Accept-Encoding", gzipped.Header().Get("Vary"))
	}

	for _, acceptEncoding := range []string{"", "gzip;q=0", "br"} {
		plain := serveFrontend(app, http.MethodGet, "/assets/bundle.abc123.js", map[string]string{"Accept-Encoding": acceptEncoding})
		if plain.Code != http.StatusOK || plain.Header().Get("Content-Encoding") != "" || plain.Body.String() != bundle {
			t.Fatalf("Accept-Encoding %q = %d with Content-Encoding %q, want the original body", acceptEncoding, plain.Code, plain.Header().Get("Content-Encoding"))
		}
		if plain.Header().Get("Vary") != "Accept-Encoding" {
			t.Errorf("Accept-Encoding %q: Vary = %q, want Accept-Encoding", acceptEncoding, plain.Header().Get("Vary"))
		}
		if plain.Header().Get("ETag") == gzipped.Header().Get("ETag") {
			t.Fatal("both encodings share an ETag")
		}
	}

	revalidated := serveFrontend(app, http.MethodGet, "/assets/bundle.abc123.js", map[string]string{
		"Accept-Encoding": "gzip",
		"If-None-Match":   gzipped.Header().Get("ETag"),
	})
	if revalidated.Code != http.StatusNotModified || revalidated.Body.Len() != 0 {
		t.Fatalf("revalidation = %d with %d body bytes, want an empty 304", revalidated.Code, revalidated.Body.Len())
	}

	for _, target := range []string{"/fonts/inter.woff2", "/assets/app.abc123.js"} {
		uncompressed := serveFrontend(app, http.MethodGet, target, map[string]string{"Accept-Encoding": "gzip"})
		if uncompressed.Header().Get("Content-Encoding") != "" || uncompressed.Header().Get("Vary") != "" {
			t.Fatalf("%s was served with Content-Encoding %q and Vary %q, want neither", target, uncompressed.Header().Get("Content-Encoding"), uncompressed.Header().Get("Vary"))
		}
	}
}

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		acceptEncoding string
		want           bool
	}{
		{"", false},
		{"gzip", true},
		{"GZIP", true},
		{"deflate, gzip;q=1.0, *;q=0.5", true},
		{"gzip; q=0.5", true},
		{"gzip;q=0", false},
		{"gzip;q=0.0", false},
		{"gzip;Q=0", false},
		{"gzip; Q = 0", false},
		{"gzip;level=1;q=0", false},
		{"gzip;level=1", true},
		{"br, deflate", false},
		{"x-gzip", false},
	}
	for _, tt := range tests {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		if tt.acceptEncoding != "" {
			request.Header.Set("Accept-Encoding", tt.acceptEncoding)
		}
		got := acceptsGzip(request)
		if got != tt.want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", tt.acceptEncoding, got, tt.want)
		}
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
