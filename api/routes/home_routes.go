package routes

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func HomeRoutes(app *fiber.App, db *gorm.DB) {
	routes := app.Group("/api/v1/recent")

	routes.Get("", func(c *fiber.Ctx) error {
		const limit = 5

		var movies []struct {
			Title string `json:"title"`
			Thumb string `json:"thumb"`
		}

		if err := db.Model(&models.Movie{}).Order("created_at desc").Limit(limit).Select("title, thumb").Find(&movies).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		}

		var albums []models.Album

		if err := db.Order("created_at desc").Preload("Musicians").Select("title, thumb").Limit(limit).Find(&albums).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})

		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":  false,
			"movies": movies,
			"albums": albums,
		})
	})
}
