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

	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/login", app.Login)

		r.Group(func(r chi.Router) {
			r.Use(app.isAuth)

			r.Get("/logout", app.Logout)

			r.Route("/users", func(r chi.Router) {
				r.Get("/me", app.GetAuthenticatedUser)
			})

			r.Route("/movies", func(r chi.Router) {
				r.Get("/latest", app.GetLatestMovies)
				r.Get("/all", app.GetAllMovies)
				r.Get("/stream/direct/{id}", app.DirectStreamMovie)
				r.Get("/{id}", app.GetMovieByID)
				r.Post("/create", app.CreateMovie)
			})
		})
	})

	// In production mode, serve the React app from the client directory
	if !app.debug {
		clientDir := http.Dir(filepath.Join(workDir, "client"))

		router.Handle("/*", http.FileServer(clientDir))

		router.NotFound(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(workDir, "client", "index.html"))
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
