package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *config) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(app.sessionLoad)

	router.Route("/api/v1/movie", func(r chi.Router) {
		r.Get("/latest", app.GetLatestMovies)
		r.Get("/{id}", app.GetMovieByID)
		r.Get("/", app.GetMoviesWithPagination)
		r.Post("/", app.CreateMovie)
	})

	router.Route("/api/v1/stream", func(r chi.Router) {
		r.Get("/video/{id}", app.DirectStreamVideo)
	})

	return router
}
