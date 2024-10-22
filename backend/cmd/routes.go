package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *config) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(app.SessionLoad)
	router.Use(app.RenewToken)

	router.Get("/api/v1/latest-movies", app.GetLatestMovies)
	router.Post("/api/v1/login", app.Login)

	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(app.isAuth)

		r.Get("/logout", app.Logout)
		r.Get("/me", app.GetAuthUser)

		// movie routes
		r.Get("/movie/{id}", app.GetMovieByID)

		// admin routes
		r.Route("/admin", func(r chi.Router) {
			// manage movies
			r.Post("/movie", app.CreateMovie)

			// manage users
			r.Get("/user", app.GetUsersWithPagination)
			r.Get("/user/{id}", app.GetUserByID)
			r.Post("/user/create", app.CreateUser)
			r.Delete("/user/{id}", app.DeleteUserByID)
		})
	})

	return router
}
