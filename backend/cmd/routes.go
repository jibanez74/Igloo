package main

import (
	"net/http"

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

	router.Get("/api/v1/latest-movies", app.GetLatestMovies)
	router.Post("/api/v1/login", app.Login)

	router.Route("/api/v1/auth", func(r chi.Router) {

		r.Get("/logout", app.Logout)

		r.Route("/movies", func(r chi.Router) {
			r.Get("/{id}", app.GetMovieByID)
			r.Get("/", app.GetMoviesWithPagination)
		})

		r.Route("/stream", func(r chi.Router) {
			r.Get("/video/simple-transcode/{id}", app.SimpleTranscodeVideoStream)
			r.Delete("/video/remove/{uuid}", app.DeleteTranscodedFile)
		})
	})

	// temp route for adding movies
	router.Post("/api/v1/movie/create", app.CreateMovie)

	// temp route for adding users
	router.Post("/api/v1/user/create", app.CreateUser)

	return router
}
