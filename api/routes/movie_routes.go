package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func MovieRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/movie")

	route.Get("/:id", func(c *fiber.Ctx) error {
		var movie models.Movie

		id := c.Params("id")

		movieId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse movie id",
			})
		}

		if err = db.First(&movie, uint(movieId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error": false,
			"movie": movie,
		})
	})

	route.Post("", func(c *fiber.Ctx) error {
		var payload struct {
			Movie models.Movie        `json:"movie"`
			Crew  []models.CrewMember `json:"crew"`
			Cast  []models.CastMember `json:"cast"`
		}

		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Create(&payload.Movie).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		tx := db.Begin()

		for _, crew := range payload.Crew {
			crew.MovieID = payload.Movie.ID

			if err := tx.Create(&crew).Error; err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		for _, cast := range payload.Cast {
			cast.MovieID = payload.Movie.ID

			if err := db.Create(&cast).Error; err != nil {
				tx.Rollback()

				return c.Status(getStatusCode(err)).JSON(fiber.Map{
					"error":   true,
					"message": err,
				})
			}
		}

		tx.Commit()

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"movie": payload.Movie,
		})
	})

	route.Put("", func(c *fiber.Ctx) error {
		var movie models.Movie

		if err := c.BodyParser(&movie); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Save(&movie).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error": false,
			"movie": movie,
		})
	})
}
