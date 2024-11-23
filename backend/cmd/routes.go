package main

import (
	"log"
	"net/http"
	"os"
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
	}

	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, "static"))
	fileServer(router, "/static", filesDir)

	// API routes
	router.Post("/api/v1/login", app.Login)

	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Route("/movies", func(r chi.Router) {
			r.Get("/latest", app.GetLatestMovies)
			r.Get("/stream/direct/{id}", app.DirectStreamMovie)
			r.Get("/{id}", app.GetMovieByID)
			r.Get("/all", app.GetAllMovies)
		})
	})

	// temp route for adding movies
	router.Post("/api/v1/movie/create", app.CreateMovie)

	// temp route for adding users
	router.Post("/api/v1/user/create", app.CreateUser)

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
		w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
		w.Header().Set("Expires", time.Now().AddDate(1, 0, 0).UTC().Format(http.TimeFormat))

		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))

		if strings.HasSuffix(r.URL.Path, ".jpg") || strings.HasSuffix(r.URL.Path, ".jpeg") {
			w.Header().Set("Content-Type", "image/jpeg")
		} else if strings.HasSuffix(r.URL.Path, ".png") {
			w.Header().Set("Content-Type", "image/png")
		}

		fs.ServeHTTP(w, r)
	})
}
