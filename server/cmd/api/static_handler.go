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

	staticDir := app.CurrentSettings().StaticDir
	fullPath := filepath.Join(staticDir, requestedPath)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(staticDir)) {
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

// loadFrontendAssets walks the webdist tree of fsys once. webdist is ~3 MB, so
// holding the decoded copies in memory is cheap next to re-reading and
// re-allocating them per request.
func loadFrontendAssets(fsys fs.FS) map[string]*frontendAsset {
	assets := make(map[string]*frontendAsset)

	walkErr := fs.WalkDir(fsys, "webdist", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		contentType := frontendContentType(path)

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

// frontendContentType resolves an embedded file's Content-Type. The web
// manifest is pinned because Go's built-in table lacks .webmanifest and the
// host's mime.types may too, and nosniff rejects the octet-stream fallback.
func frontendContentType(path string) string {
	ext := filepath.Ext(path)
	if strings.EqualFold(ext, ".webmanifest") {
		return "application/manifest+json"
	}

	if contentType := mime.TypeByExtension(ext); contentType != "" {
		return contentType
	}

	return "application/octet-stream"
}

func (app *Application) frontendAssetFor(fsPath string) (*frontendAsset, bool) {
	app.frontendAssetsOnce.Do(func() {
		app.frontendAssets = loadFrontendAssets(app.FrontendAssets)
	})

	asset, ok := app.frontendAssets[fsPath]
	return asset, ok
}

// frontendCacheControl picks the cache policy for an embedded file. Only
// Vite's content-hashed output under assets/ is immutable. HTML must never be
// stored so deploys are picked up, and the remaining fixed-name files
// (manifest, icons, robots.txt, fonts) revalidate against their ETag.
func frontendCacheControl(fsPath string) string {
	switch {
	case strings.HasSuffix(fsPath, ".html"):
		return "no-cache, no-store, must-revalidate"
	case strings.HasPrefix(fsPath, "webdist/assets/"):
		return "public, max-age=31536000"
	default:
		return "no-cache"
	}
}

func serveFrontendAsset(w http.ResponseWriter, r *http.Request, asset *frontendAsset, fsPath string) {
	w.Header().Set("Cache-Control", frontendCacheControl(fsPath))
	w.Header().Set("Content-Type", asset.contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Igloo is a private server: keep every page and asset out of search
	// indexes, including for crawlers that never run the SPA's robots meta.
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("ETag", asset.etag)

	// Embedded files carry no modtime; the ETag drives conditional requests.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(asset.content))
}

// ServeFrontend serves the React SPA from embedded files, or redirects to the Vite dev
// server when VITE_DEV_SERVER is set (e.g. VITE_DEV_SERVER=http://localhost:3000 for make dev).
func (app *Application) ServeFrontend(w http.ResponseWriter, r *http.Request) {
	viteURL := viteDevServerURL()
	if viteURL != "" {
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

	if asset, ok := app.frontendAssetFor(fsPath); ok {
		serveFrontendAsset(w, r, asset, fsPath)
		return
	}

	// A directory request serves its own index.html when one exists.
	if asset, ok := app.frontendAssetFor(fsPath + "/index.html"); ok {
		serveFrontendAsset(w, r, asset, fsPath+"/index.html")
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
	asset, ok := app.frontendAssetFor("webdist/index.html")
	if !ok {
		app.Logger.Error("failed to find index.html in embedded filesystem")
		http.Error(w, "Frontend not found. Please build the web application and rebuild the binary.", http.StatusNotFound)
		return
	}

	serveFrontendAsset(w, r, asset, "webdist/index.html")
}
