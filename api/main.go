package main

import (
	"igloo/database"
	"igloo/handlers"

	"github.com/gofiber/fiber/v2"
)

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		panic(err)
	}

	h := handlers.NewAppHandler(db)

	app := fiber.New()

	// musician routes
	app.Get("/api/v1/musician", h.GetMusicianWithPagination)
	app.Get("/api/v1/musician/name/{name}", h.GetMusicianByName)
	app.Get("/api/v1/musician/{id}", h.GetMusicianById)
	app.Post("/api/v1/musician", h.CreateMusician)
	app.Delete("/api/v1/musician/{id}", h.DeleteMusician)

	app.Listen(":8080")
}
