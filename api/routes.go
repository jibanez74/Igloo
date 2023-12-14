package main

import (
	"igloo/custom_middleware"
	"igloo/handlers"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func routes(h *handlers.AppHandler) http.Handler {
	mux := chi.NewRouter()

	mux.Use(middleware.Recoverer)
	mux.Use(middleware.Logger)
	mux.Use(custom_middleware.EnableCORS)

	// user routes
	mux.Post("/api/v1/auth", h.UserHandler.Authenticate)
	mux.Post("/api/v1/admin/users", h.UserHandler.CreateUser)

	// music mood routes
	mux.Get("/api/v1/music-moods/{id}", h.MoodHandler.GetMoodByID)
	mux.Post("/api/v1/admin/music-moods", h.MoodHandler.FindOrCreateByTag)

	// music genre routes
	mux.Get("/api/v1/music-genre/{id}", h.MusicGenreHandler.FindMusicGenreByID)
	mux.Post("/api/v1/music-genre", h.MusicGenreHandler.FindOrCreateByTag)

	// musician routes
	mux.Get("/api/v1/musician/{id}", h.MusicianHandler.GetMusicianByID)
	mux.Get("/api/v1/musician/name/{name}", h.MusicianHandler.GetMusicianByName)
	mux.Get("/api/v1/musician", h.MusicianHandler.GetMusicians)
	mux.Post("/api/v1/musician", h.MusicianHandler.CreateMusician)

	// album
	mux.Get("/api/v1/albums/{id}", h.AlbumHandler.GetAlbumByID)
	mux.Post("/api/v1/admin/albums", h.AlbumHandler.FindOrCreateByTitle)

	mux.Get("/api/v1/tracks/{id}", h.TrackHandler.GetTrackByID)
	mux.Put("/api/v1/admin/tracks", h.TrackHandler.CreateTrack)

	return mux
}
