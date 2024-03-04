package routes

import (
  "igloo/models"
  "strconv"

  "github.com/gofiber/fiber/v2"
  "gorm.io/gorm"
)

func AlbumRoutes(app *fiber.App, db *gorm.DB) {
  route := app.Group("/api/v1/album")

  route.Get("/title/:title", func(c *fiber.Ctx) error {
    var album models.Album

    title := c.Params("title")

    if err := db.Where("title = ?", title).First(&album).Error; err != nil {
      return c.Status(getStatusCode(err)).JSON(fiber.Map{
        "error":   false,
        "message": err,
      })
    }

    return c.Status(fiber.StatusOK).JSON(fiber.Map{
      "error": false,
      "album": album,
    })
  })

  route.Get("/:id", func(c *fiber.Ctx) error {
    var album models.Album

    id := c.Params("id")

    albumId, err := strconv.ParseUint(id, 10, 64)
    if err != nil {
      return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
        "error":   true,
        "message": "Unable to parse album id",
      })
    }

    if err = db.First(&album, uint(albumId)).Error; err != nil {
      return c.Status(getStatusCode(err)).JSON(fiber.Map{
        "error":   false,
        "message": err,
      })
    }

    return c.Status(fiber.StatusOK).JSON(fiber.Map{
      "error": false,
      "album": album,
    })
  })

  route.Post("", func(c *fiber.Ctx) error {
    var album models.Album

    if err := c.BodyParser(&album); err != nil {
      return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
        "error":   true,
        "message": err,
      })
    }

    if err := db.Where("title = ?", album.Title).FirstOrCreate(&album).Error; err != nil {
      return c.Status(getStatusCode(err)).JSON(fiber.Map{
        "error":   true,
        "message": err,
      })
    }

    return c.Status(fiber.StatusCreated).JSON(fiber.Map{
      "error": false,
      "album": album,
    })
  })
}
