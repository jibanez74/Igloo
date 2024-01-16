package main

import (
	"fmt"
	"igloo/database"
	"igloo/handlers"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const port = 8080

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		log.Fatal(err)
	}

	appHandlers := handlers.NewAppHandlers(db)

	mux := chi.NewRouter()

	mux.Use(middleware.Recoverer)

	// music genre routes
	mux.Get("/api/v1/music-genre/id/{id}", appHandlers.GetMusicGenreByID)
	mux.Get("/api/v1/music-genre/tag/{tag}", appHandlers.GetMusicGenreByTag)
	mux.Get("/api/v1/music-genre", appHandlers.GetMusicGenres)
	mux.Post("/api/v1/music-genre", appHandlers.FindOrCreateMusicGenre)

	// musician routes
	mux.Get("/api/v1/musician/id/{id}", appHandlers.GetMusicianByID)
	mux.Get("/api/v1/musician/name/{name}", appHandlers.GetMusicianByName)
	mux.Get("/api/v1/musician", appHandlers.GetMusicians)
	mux.Post("/api/v1/musician", appHandlers.CreateMusician)


	log.Println("Starting application on port", port)

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		log.Fatal(err)
	}
}
