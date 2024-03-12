package routes

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func AlbumRoutes(app *fiber.App, db *gorm.DB) {
	routes := app.Group("/api/v1/album")

	routes.Post("", func(c *fiber.Ctx) error {
		var album models.Album

		err := c.BodyParser(&album)
		if err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		tx := db.Begin()

		for _, genre := range album.Genres {
			err = db.FirstOrCreate(&genre).Error
			if err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		err = db.Create(&album).Error
		if err != nil {
			tx.Rollback()

			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		tx.Commit()

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"album": album,
		})
	})

}
