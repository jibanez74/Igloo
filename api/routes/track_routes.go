package routes

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func TrackRoutes(app *fiber.App, db *gorm.DB) {
	routes := app.Group("/api/v1/track")

	routes.Post("", func(c *fiber.Ctx) error {
		var track models.Track

		err := c.BodyParser(&track)
		if err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		tx := db.Begin()

		for _, genre := range track.Genres {
			err = db.FirstOrCreate(&genre).Error
			if err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		for _, mood := range track.Moods {
			err = db.FirstOrCreate(&mood).Error
			if err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		err = db.Create(&track).Error
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
			"track": track,
		})
	})
}
