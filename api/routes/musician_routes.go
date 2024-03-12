package routes

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func MusicianRoutes(app *fiber.App, db *gorm.DB) {
	routes := app.Group("/api/v1/musician")

	routes.Post("/name", func(c *fiber.Ctx) error {
		var payload struct {
			Name string `json:"name"`
		}

		err := c.BodyParser(&payload)
		if err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		var musician models.Musician

		err = db.Where("name = ?", payload.Name).First(&musician).Error
		if err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":    false,
			"musician": musician,
		})
	})

	routes.Post("", func(c *fiber.Ctx) error {
		var musician models.Musician

		err := c.BodyParser(&musician)
		if err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		tx := db.Begin()

		for _, genre := range musician.Genres {
			err = db.FirstOrCreate(&genre).Error
			if err != nil {
				tx.Rollback()
				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		err = tx.Create(&musician).Error
		if err != nil {
			tx.Rollback()
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		tx.Commit()

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error":    false,
			"musician": musician,
		})
	})
}
