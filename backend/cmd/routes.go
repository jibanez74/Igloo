package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *config) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	router.Route("/api/v1/artist", func(r chi.Router) {
		r.Post("/create", app.FindOrCreateArtist)
	})

	router.Route("/api/v1/genre", func(r chi.Router) {
		r.Post("/create", app.FindOrCreateGenre)
	})

	router.Route("/api/v1/studio", func(r chi.Router) {
		r.Post("/create", app.FindOrCreateStudio)
	})

	router.Route("/api/v1/movie", func(r chi.Router) {
		r.Get("/latest", app.GetLatestMovies)
		r.Get("/all", app.GetMovies)
		r.Get("/{id}", app.GetMovieByID)
		r.Post("/create", app.CreateMovie)
	})

	return router
}
