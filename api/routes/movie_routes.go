package routes

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func MovieRoutes(app *fiber.App, db *gorm.DB) {
	routes := app.Group("/api/v1/movie")

	routes.Post("", func(c *fiber.Ctx) error {
		var movie models.Movie

		err := c.BodyParser(&movie)
		if err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		tx := db.Begin()

		for _, genre := range movie.Genres {
			err = db.FirstOrCreate(&genre).Error
			if err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		for _, studio := range movie.Studios {
			err = db.FirstOrCreate(&studio).Error
			if err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		for _, artist := range movie.Artists {
			err = db.FirstOrCreate(&artist).Error
			if err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		err = db.Create(&movie).Error
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
			"movie": movie,
		})

	})
}
