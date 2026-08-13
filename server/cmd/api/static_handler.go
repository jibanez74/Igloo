package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// Missing embedded assets must 404; index.html fallback breaks module MIME checks.
func embeddedPathLooksLikeStaticAsset(relPath string) bool {
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".js", ".mjs", ".cjs", ".css", ".map":
		return true
	case ".woff", ".woff2", ".ttf", ".otf", ".eot":
		return true
	case ".ico", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".avif":
		return true
	case ".webmanifest", ".json":
		return true
	default:
		return false
	}
}

// ServeStaticFiles serves configured static files with traversal protection.
func (app *Application) ServeStaticFiles(w http.ResponseWriter, r *http.Request) {
	requestedPath := chi.URLParam(r, "*")

	if strings.Contains(requestedPath, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(app.Settings.StaticDir, requestedPath)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(app.Settings.StaticDir)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			app.Logger.Error("failed to stat static file", "error", err, "path", fullPath)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}

		return
	}

	if info.IsDir() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		app.Logger.Error("failed to open static file", "error", err, "path", fullPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	ext := filepath.Ext(fullPath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// frontendAsset is one embedded SPA file, read and fingerprinted once so the
// request path serves from memory with ETag revalidation instead of
// re-reading the embedded filesystem on every hit.
type frontendAsset struct {
	content     []byte
	contentType string
	etag        string
}

var (
	frontendAssetsOnce sync.Once
	frontendAssets     map[string]*frontendAsset
)

// loadFrontendAssets walks the embedded webdist once. webdist is ~3 MB, so
// holding the decoded copies in memory is cheap next to re-reading and
// re-allocating them per request.
func loadFrontendAssets() map[string]*frontendAsset {
	assets := make(map[string]*frontendAsset)

	walkErr := fs.WalkDir(FrontendFS, "webdist", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		content, err := fs.ReadFile(FrontendFS, path)
		if err != nil {
			return err
		}

		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		sum := sha256.Sum256(content)
		assets[path] = &frontendAsset{
			content:     content,
			contentType: contentType,
			etag:        `"` + hex.EncodeToString(sum[:16]) + `"`,
		}
		return nil
	})
	if walkErr != nil {
		// A missing or unreadable webdist degrades to the same "frontend not
		// found" responses the per-request reads produced.
		return assets
	}

	return assets
}

func frontendAssetFor(fsPath string) (*frontendAsset, bool) {
	frontendAssetsOnce.Do(func() {
		frontendAssets = loadFrontendAssets()
	})

	asset, ok := frontendAssets[fsPath]
	return asset, ok
}

func serveFrontendAsset(w http.ResponseWriter, r *http.Request, asset *frontendAsset, isHTML bool) {
	// Hashed assets are immutable; HTML must revalidate so deploys are
	// picked up. Both get an ETag so revalidation can answer 304.
	if isHTML {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	}

	w.Header().Set("Content-Type", asset.contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("ETag", asset.etag)

	// Embedded files carry no modtime; the ETag drives conditional requests.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(asset.content))
}

// ServeFrontend serves the React SPA from embedded files, or redirects to the Vite dev
// server when VITE_DEV_SERVER is set (e.g. VITE_DEV_SERVER=http://localhost:3000 for make dev).
func (app *Application) ServeFrontend(w http.ResponseWriter, r *http.Request) {
	if viteURL := os.Getenv("VITE_DEV_SERVER"); viteURL != "" {
		viteURL = strings.TrimSuffix(viteURL, "/")
		target := viteURL + r.URL.Path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		return
	}

	requestedPath := chi.URLParam(r, "*")
	if requestedPath == "" {
		requestedPath = strings.TrimPrefix(r.URL.Path, "/")
	}
	if requestedPath == "" {
		requestedPath = "index.html"
	}

	if strings.Contains(requestedPath, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	requestedPath = filepath.Clean(requestedPath)

	// embed.FS uses webdist/... paths with forward slashes.
	fsPath := filepath.Join("webdist", requestedPath)
	fsPath = filepath.ToSlash(fsPath)

	if asset, ok := frontendAssetFor(fsPath); ok {
		serveFrontendAsset(w, r, asset, strings.HasSuffix(fsPath, ".html"))
		return
	}

	// A directory request serves its own index.html when one exists.
	if asset, ok := frontendAssetFor(fsPath + "/index.html"); ok {
		serveFrontendAsset(w, r, asset, true)
		return
	}

	if embeddedPathLooksLikeStaticAsset(requestedPath) {
		app.Logger.Warn(
			"embedded frontend asset missing; rebuild web, copy to cmd/api/webdist, then rebuild the binary",
			"path", fsPath,
		)
		http.Error(
			w,
			"Not Found: embedded static asset missing. From the repo: run make build from server/ to embed the web app.",
			http.StatusNotFound,
		)
		return
	}

	// SPA fallback: any other path serves the root index.html.
	asset, ok := frontendAssetFor("webdist/index.html")
	if !ok {
		app.Logger.Error("failed to find index.html in embedded filesystem")
		http.Error(w, "Frontend not found. Please build the web application and rebuild the binary.", http.StatusNotFound)
		return
	}

	serveFrontendAsset(w, r, asset, true)
}
