package main

import (
  "igloo/database"
  "igloo/handlers"
  "igloo/custom_middleware"
  "net/http"

  "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
  // initializes the db and runs migrations
  db, err := database.InitDatabase()
  if err != nil {
    panic(err)
  }

  // initializes the app handler for api routes
  appHandler := handlers.NewAppHandler(db)

  // list of middleware
  mux := chi.NewRouter()
  mux.Use(middleware.Recoverer)
  mux.Use(middleware.Logger)
  mux.Use(custom_middleware.EnableCORS)

  // Album routes
  mux.Get("/api/v1/musician/{id}", appHandler.FindMusicianByID)
  mux.Get("/api/v1/musician/name/{name}", appHandler.FindMusicianByName)
  mux.Get("/api/v1/musician", appHandler.GetMusicians)
  mux.Post("/api/v1/musician", appHandler.CreateMusician)

  err = http.ListenAndServe(":8080", mux)
  if err != nil {
    panic(err)
  }
}
