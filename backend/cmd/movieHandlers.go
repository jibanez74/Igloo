package main

import (
  "igloo/cmd/database/models"
  "igloo/cmd/helpers"
  "igloo/cmd/repository"
  "os"
  "strconv"

  "github.com/gofiber/fiber/v2"
)

func (app *config) GetLatestMovies(c *fiber.Ctx) error {
  var movies []repository.SimpleMovie

  status, err := app.Repo.GetLatestMovies(&movies)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  return c.Status(status).JSON(fiber.Map{
    "movies": movies,
  })
}

func (app *config) GetMovieByID(c *fiber.Ctx) error {
  id, err := strconv.ParseUint(c.Params("id"), 10, 64)
  if err != nil {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  var movie models.Movie
  movie.ID = uint(id)

  status, err := app.Repo.GetMovieByID(&movie)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  return c.Status(status).JSON(fiber.Map{
    "movie": movie,
  })
}

func (app *config) CreateMovie(c *fiber.Ctx) error {
  var movie models.Movie

  err := c.BodyParser(&movie)
  if err != nil {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  if movie.FilePath == "" {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": "movie file path is required",
    })
  }

  if movie.TmdbID != "" {
    err = app.Tmdb.GetTmdbMovieByID(&movie)
    if err != nil {
      return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
        "error": err.Error(),
      })
    }
  }

  err = helpers.GetMovieMetadata(&movie)
  if err != nil {
    return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  status, err := app.Repo.CreateMovie(&movie)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  return c.Status(status).JSON(fiber.Map{
    "movie": movie,
  })
}

func (app *config) DirectStreamMovie(c *fiber.Ctx) error {
  id, err := strconv.ParseUint(c.Params("id"), 10, 64)
  if err != nil {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  var movie models.Movie
  movie.ID = uint(id)

  status, err := app.Repo.GetMovieByID(&movie)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  status, err = helpers.CheckFileExist(movie.FilePath)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  file, err := os.Open(movie.FilePath)
  if err != nil {
    return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
      "error": err.Error(),
    })
  }
  defer file.Close()

  c.Set("Content-Type", movie.ContentType)
  c.Set("Content-Length", strconv.FormatInt(movie.Size, 10))
  c.Set("Content-Disposition", "inline; filename="+movie.Title)
  c.Set("Accept-Ranges", "bytes")

  return c.SendFile(movie.FilePath)
}

func (app *config) GetAllMovies(c *fiber.Ctx) error {
  var movies []repository.SimpleMovie

  status, err := app.Repo.GetAllMovies(&movies)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": err.Error(),
    })
  }

  return c.Status(status).JSON(fiber.Map{
    "movies": movies,
    "count":  len(movies),
  })
}
