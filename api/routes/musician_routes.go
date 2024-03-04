package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func MusicianRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/musician")

	route.Get("/:id", func(c *fiber.Ctx) error {
		var musician models.Musician

		id := c.Params("id")

		musicianId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse musician id",
			})
		}

		if err = db.First(&musician, uint(musicianId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":    false,
			"musician": musician,
		})
	})

	route.Post("/name", func(c *fiber.Ctx) error {
		var musician models.Musician
		var payload struct {
			Name string `json:"name"`
		}

		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("name = ?", payload.Name).First(&musician).Error; err != nil {
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

	route.Post("", func(c *fiber.Ctx) error {
		var musician models.Musician

		if err := c.BodyParser(&musician); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("name = ?", musician.Name).FirstOrCreate(&musician).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":    true,
				"message":  err,
				"musician": musician,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error":    false,
			"musician": musician,
		})
	})
}
