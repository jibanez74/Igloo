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

// ServeStaticFiles serves static files from the configured static directory.
// It sets appropriate cache headers and prevents directory traversal attacks.
func (app *Application) ServeStaticFiles(w http.ResponseWriter, r *http.Request) {
	// Get the file path from the URL
	requestedPath := chi.URLParam(r, "*")

	// Prevent directory traversal attacks
	if strings.Contains(requestedPath, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Build the full file path
	fullPath := filepath.Join(app.Settings.StaticDir, requestedPath)

	// Clean the path to prevent any remaining traversal attempts
	fullPath = filepath.Clean(fullPath)

	// Verify the path is still within the static directory
	if !strings.HasPrefix(fullPath, filepath.Clean(app.Settings.StaticDir)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check if file exists
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

	// Don't serve directories
	if info.IsDir() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Open the file
	file, err := os.Open(fullPath)
	if err != nil {
		app.Logger.Error("failed to open static file", "error", err, "path", fullPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Determine content type from file extension
	ext := filepath.Ext(fullPath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Serve the file
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// ServeFrontend serves the React SPA (Single Page Application) from embedded files.
// It serves static assets from the embedded FrontendFS and falls back to index.html
// for client-side routing (SPA behavior).
func (app *Application) ServeFrontend(w http.ResponseWriter, r *http.Request) {
	// Get the requested path from the URL
	requestedPath := chi.URLParam(r, "*")

	// If no path specified, default to index.html
	if requestedPath == "" {
		requestedPath = "index.html"
	}

	// Prevent directory traversal attacks
	if strings.Contains(requestedPath, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Clean the path
	requestedPath = filepath.Clean(requestedPath)

	// Build the path within the embedded filesystem
	// The embed path is webdist, so files are at webdist/... in the FS
	fsPath := filepath.Join("webdist", requestedPath)
	fsPath = filepath.ToSlash(fsPath) // Use forward slashes for embed.FS

	// Try to open the file from embedded filesystem
	file, err := FrontendFS.Open(fsPath)
	if err != nil {
		// File doesn't exist - serve index.html for SPA routing
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

	// Get file info for content type and caching
	info, err := file.Stat()
	if err != nil {
		app.Logger.Error("failed to stat embedded file", "error", err, "path", fsPath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Variable to hold file content and info
	var content []byte
	var fileInfo fs.FileInfo

	// If it's a directory, serve index.html
	if info.IsDir() {
		indexPath := filepath.Join(fsPath, "index.html")
		indexPath = filepath.ToSlash(indexPath)
		indexContent, err := fs.ReadFile(FrontendFS, indexPath)
		if err != nil {
			// Directory doesn't have index.html, try root index.html
			rootIndexPath := "webdist/index.html"
			indexContent, err = fs.ReadFile(FrontendFS, rootIndexPath)
			if err != nil {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			// Get file info for the root index.html
			indexFile, _ := FrontendFS.Open(rootIndexPath)
			if statInfo, err := indexFile.Stat(); err == nil {
				fileInfo = statInfo
			} else {
				fileInfo = info // Fallback to directory info
			}
			indexFile.Close()
			content = indexContent
		} else {
			// Get file info for the directory's index.html
			indexFile, _ := FrontendFS.Open(indexPath)
			if statInfo, err := indexFile.Stat(); err == nil {
				fileInfo = statInfo
			} else {
				fileInfo = info // Fallback to directory info
			}
			indexFile.Close()
			content = indexContent
		}
	} else {
		// Read file content (fs.File doesn't implement io.ReadSeeker)
		content, err = fs.ReadFile(FrontendFS, fsPath)
		if err != nil {
			app.Logger.Error("failed to read embedded file", "error", err, "path", fsPath)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		fileInfo = info
	}

	// Determine content type from file extension
	ext := filepath.Ext(fileInfo.Name())
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set appropriate cache headers based on file type
	// Static assets (JS, CSS, images) should be cached, HTML should not
	if ext == ".html" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year for static assets
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Serve the file content
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}
