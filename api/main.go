package main

import (
	"igloo/database"
	"igloo/handlers"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		panic(err)
	}

	// fiber configuration
	app := fiber.New()
	app.Use(recover.New())

	h := handlers.NewAppHandler(db)

	// musician routes
	app.Get("/api/v1/musician", h.GetMusicianWithPagination)
	app.Get("/api/v1/musician/name/{name}", h.GetMusicianByName)
	app.Get("/api/v1/musician/{id}", h.GetMusicianById)
	app.Post("/api/v1/musician", h.CreateMusician)
	app.Delete("/api/v1/musician/{id}", h.DeleteMusician)

	// album routes
	app.Get("/api/v1/album/title/{title}", h.GetAlbumByTitle)
	app.Get("/api/v1/album/{id}", h.GetAlbumById)
	app.Get("/api/v1/album", h.GetAlbumsWithPagination)
	app.Post("/api/v1/album", h.CreateAlbum)

	// music genre routes
	app.Get("/api/v1/music-genre", h.GetMusicGenres)
	app.Post("/api/v1/music-genre", h.FindOrCreateMusicGenre)

	// album routes
	app.Get("/api/v1/album", h.GetAlbumsWithPagination)
	app.Get("/api/v1/album/title/{title}", h.GetAlbumByTitle)
	app.Get("/api/v1/album/{id}", h.GetAlbumById)
	app.Post("/api/v1/album", h.CreateAlbum)

	app.Listen(":8080")
}
