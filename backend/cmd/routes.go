package main

import (
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *config) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(app.sessionLoad)
	router.Use(app.reloadSessionToken)

	debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		panic(err)
	}

	if debug {
		router.Use(middleware.Logger)
		router.Use(middleware.RealIP)
		router.Use(middleware.RequestID)
	}

	router.Get("/api/v1/latest-movies", app.GetLatestMovies)
	router.Post("/api/v1/login", app.Login)

	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(app.isAuth)

		r.Get("/logout", app.Logout)

		r.Route("/users", func(r chi.Router) {
			r.Get("/me", app.GetAuthUser)
		})

		r.Route("/movies", func(r chi.Router) {
			r.Get("/{id}", app.GetMovieByID)
			r.Get("/", app.GetMoviesWithPagination)
		})

		r.Route("/stream", func(r chi.Router) {
			r.Get("/video/direct/{id}", app.DirectPlayVideo)
		})
	})

	// temp route for adding movies
	router.Post("/api/v1/movie/create", app.CreateMovie)

	// temp route for adding users
	router.Post("/api/v1/user/create", app.CreateUser)

	return router
}
