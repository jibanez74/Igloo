package main

import (
	"igloo/database"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const PORT = ":8080"

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		panic("Unable to connect to db")
	}

	app := fiber.New()
	app.Use(recover.New())
	app.Use(cors.New())

	movieRoutes(app, db)

	app.Listen(PORT)
}
