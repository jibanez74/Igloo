package main

import (
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	file, err := FrontendFS.Open(fsPath)
	if err != nil {
		if embeddedPathLooksLikeStaticAsset(requestedPath) {
			app.Logger.Warn(
				"embedded frontend asset missing; rebuild web, copy to cmd/api/webdist, then rebuild the binary",
				"path", fsPath,
			)
			http.Error(
				w,
				"Not Found: embedded static asset missing. From the repo: build the web app, copy dist to server/cmd/api/webdist, then run go build (see server/Makefile target build-full).",
				http.StatusNotFound,
			)
			return
		}

		indexPath := "webdist/index.html"
		content, err := fs.ReadFile(FrontendFS, indexPath)
		if err != nil {
			app.Logger.Error("failed to find index.html in embedded filesystem", "error", err)
			http.Error(w, "Frontend not found. Please build the web application and rebuild the binary.", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusOK)
		w.Write(content)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		app.Logger.Error("failed to stat embedded file", "error", err, "path", fsPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var content []byte
	var fileInfo fs.FileInfo

	if info.IsDir() {
		indexPath := filepath.Join(fsPath, "index.html")
		indexPath = filepath.ToSlash(indexPath)
		indexContent, err := fs.ReadFile(FrontendFS, indexPath)
		if err != nil {
			rootIndexPath := "webdist/index.html"
			indexContent, err = fs.ReadFile(FrontendFS, rootIndexPath)
			if err != nil {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			indexFile, _ := FrontendFS.Open(rootIndexPath)
			if statInfo, err := indexFile.Stat(); err == nil {
				fileInfo = statInfo
			} else {
				fileInfo = info
			}
			indexFile.Close()
			content = indexContent
		} else {
			indexFile, _ := FrontendFS.Open(indexPath)
			if statInfo, err := indexFile.Stat(); err == nil {
				fileInfo = statInfo
			} else {
				fileInfo = info
			}
			indexFile.Close()
			content = indexContent
		}
	} else {
		content, err = fs.ReadFile(FrontendFS, fsPath)
		if err != nil {
			app.Logger.Error("failed to read embedded file", "error", err, "path", fsPath)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		fileInfo = info
	}

	ext := filepath.Ext(fileInfo.Name())
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Static assets should be cached; HTML should not.
	if ext == ".html" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}
