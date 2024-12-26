package main

import (
  "os"
  "path/filepath"
  "time"

  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/logger"
  "github.com/gofiber/fiber/v2/middleware/recover"
)

func (app *config) run() error {
  f := fiber.New(fiber.Config{
    AppName:               "Igloo API",
    ReadTimeout:           10 * time.Second,
    WriteTimeout:          10 * time.Second,
    IdleTimeout:           10 * time.Second,
    DisableStartupMessage: app.debug,
    ErrorHandler: func(c *fiber.Ctx, err error) error {
      code := fiber.StatusInternalServerError

      if e, ok := err.(*fiber.Error); ok {
        code = e.Code
      }

      return c.Status(code).JSON(fiber.Map{
        "error": err.Error(),
      })
    },
  })

  f.Use(recover.New(recover.Config{
    EnableStackTrace: app.debug,
  }))

  f.Use(logger.New(logger.Config{
    Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
    TimeFormat: "2006-01-02 15:04:05",
    TimeZone:   "Local",
  }))

  staticDir := filepath.Join(app.workDir, "static")
  f.Static("/static", staticDir, fiber.Static{
    Compress:      true,
    ByteRange:     true,
    Browse:        false,
    CacheDuration: 24 * time.Hour,
  })

  auth := f.Group("/api/v1/auth")
  auth.Post("/login", app.Login)
  auth.Get("/logout", app.Logout)

  movies := f.Group("/api/v1/movies")
  movies.Get("/all", app.GetAllMovies)
  movies.Get("/latest", app.GetLatestMovies)
  movies.Post("/create", app.CreateMovie)

  users := f.Group("/api/v1/users")
  users.Post("/create", app.CreateUser)

  if !app.debug {
    clientDir := filepath.Join(app.workDir, "cmd", "client")
    assetsDir := filepath.Join(clientDir, "assets")

    f.Static("/assets", assetsDir, fiber.Static{
      Compress:      true,
      ByteRange:     true,
      Browse:        false,
      CacheDuration: 24 * time.Hour,
    })

    f.Get("*", func(c *fiber.Ctx) error {
      indexFile := filepath.Join(clientDir, "index.html")

      _, err := os.Stat(indexFile)
      if err != nil {
        panic(err)
      }

      c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
      c.Set("Pragma", "no-cache")
      c.Set("Expires", "0")

      return c.SendFile(indexFile)
    })
  }

  return f.Listen(app.port)
}
