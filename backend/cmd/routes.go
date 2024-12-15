package main

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *config) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	if app.debug {
		router.Use(middleware.Logger)
		router.Use(middleware.RealIP)
		router.Use(middleware.RequestID)

		router.Post("/api/v1/users/create", app.CreateUser)
		router.Post("/api/v1/movies/create", app.CreateMovie)
	}

	filesDir := http.Dir(filepath.Join(app.workDir, "static"))
	fileServer(router, "/api/v1/static", filesDir)

	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/login", app.Login)

		r.Group(func(r chi.Router) {
			// r.Use(app.isAuth)

			r.Get("/logout", app.Logout)

			r.Route("/users", func(r chi.Router) {
				r.Get("/me", app.GetAuthenticatedUser)
			})

			r.Route("/movies", func(r chi.Router) {
				r.Get("/latest", app.GetLatestMovies)
				r.Get("/all", app.GetAllMovies)
				r.Get("/stream/direct/{id}", app.DirectStreamMovie)
				r.Get("/{id}", app.GetMovieByID)
			})
		})
	})

	if !app.debug {
		assetsDir := http.Dir(filepath.Join(app.workDir, "cmd", "client", "assets"))
		fileServer(router, "/assets", assetsDir)

		router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(app.workDir, "cmd", "client", "index.html"))
		})
	}

	return router
}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		log.Printf("Warning: FileServer path contains URL parameters: %s", path)
		return
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		// Set different cache durations based on file type
		if strings.HasSuffix(r.URL.Path, ".jpg") ||
			strings.HasSuffix(r.URL.Path, ".jpeg") ||
			strings.HasSuffix(r.URL.Path, ".png") {
			// Cache images for 1 year
			w.Header().Set("Cache-Control", "public, max-age=31536000") // 31536000 seconds = 1 year
			w.Header().Set("Expires", time.Now().AddDate(1, 0, 0).UTC().Format(http.TimeFormat))
		} else if strings.HasSuffix(r.URL.Path, ".css") ||
			strings.HasSuffix(r.URL.Path, ".js") {
			// Cache CSS and JS for 24 hours
			w.Header().Set("Cache-Control", "public, max-age=86400") // 86400 seconds = 24 hours
			w.Header().Set("Expires", time.Now().AddDate(0, 0, 1).UTC().Format(http.TimeFormat))
		} else {
			// Default: no cache for other files
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}

		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))

		// Set appropriate content types
		if strings.HasSuffix(r.URL.Path, ".jpg") || strings.HasSuffix(r.URL.Path, ".jpeg") {
			w.Header().Set("Content-Type", "image/jpeg")
		} else if strings.HasSuffix(r.URL.Path, ".png") {
			w.Header().Set("Content-Type", "image/png")
		} else if strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		}

		fs.ServeHTTP(w, r)
	})
}
