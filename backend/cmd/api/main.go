package main

import (
  "igloo/cmd/internal/database"
  "igloo/cmd/internal/database/models"
  "igloo/cmd/internal/ffmpeg"
  "igloo/cmd/internal/tmdb"
  "log"
  "os"
  "time"

  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/logger"
  "github.com/gofiber/fiber/v2/middleware/recover"
  "github.com/gofiber/fiber/v2/middleware/session"

  "gorm.io/gorm"
)

type config struct {
  db       *gorm.DB
  settings *models.GlobalSettings
  workDir  string
  tmdb     tmdb.Tmdb
  ffmpeg   ffmpeg.FFmpeg
  store    *session.Store
}

func main() {
  var app config

  db, err := database.New()
  if err != nil {
    log.Fatal("Failed to initialize database:", err)
  }
  app.db = db

  app.settings = &models.GlobalSettings{}
  err = db.First(app.settings).Error
  if err != nil {
    log.Fatal("Failed to load settings:", err)
  }

  app.store = session.New(session.Config{
    KeyLookup:      "cookie:session_id",
    CookieName:     "session_id",
    CookieSecure:   !app.settings.Debug,
    CookieHTTPOnly: true,
    CookieSameSite: "Strict",
    Expiration:     24 * time.Hour,
  })

  app.ffmpeg, err = ffmpeg.New(app.settings.Ffmpeg)
  if err != nil {
    log.Fatal("Failed to initialize ffmpeg:", err)
  }

  app.workDir, err = os.Getwd()
  if err != nil {
    log.Fatal("Failed to get working directory:", err)
  }

  app.tmdb, err = tmdb.New(&app.settings.TmdbKey)
  if err != nil {
    log.Fatal("Failed to initialize TMDB client:", err)
  }

  f := fiber.New(fiber.Config{
    AppName:               "Igloo API",
    ReadTimeout:           10 * time.Second,
    WriteTimeout:          10 * time.Second,
    IdleTimeout:           10 * time.Second,
    DisableStartupMessage: app.settings.Debug,
    ErrorHandler: func(c *fiber.Ctx, err error) error {
      code := fiber.StatusInternalServerError

      e, ok := err.(*fiber.Error)
      if ok {
        code = e.Code
      }

      return c.Status(code).JSON(fiber.Map{
        "error": err.Error(),
      })
    },
  })

  f.Use(recover.New(recover.Config{
    EnableStackTrace: app.settings.Debug,
  }))

  f.Use(logger.New(logger.Config{
    Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
    TimeFormat: "2006-01-02 15:04:05",
    TimeZone:   "Local",
  }))

  f.Static("/api/v1/static", app.settings.StaticDir, fiber.Static{
    Compress:      true,
    ByteRange:     true,
    Browse:        false,
    CacheDuration: 24 * time.Hour,
  })

  auth := f.Group("/api/v1/auth")
  auth.Get("/me", app.requireAuth, app.getAuthUser)
  auth.Post("/login", app.login)
  auth.Post("/logout", app.logout)

  movies := f.Group("/api/v1/movies")
  movies.Get("/", app.GetAllMovies)
  movies.Get("/latest", app.getLatestMovies)
  movies.Get("/:id", app.GetMovieBydID)
  movies.Get("/:id/stream", app.directStreamMovie)
  movies.Post("/create", app.createMovie)

  users := f.Group("/api/v1/users")
  users.Get("/", app.GetUsers)
  users.Post("/create", app.CreateUser)
  users.Post("/upload-photo", app.UploadUserPhoto)

  // Serve uploaded files statically
  f.Static("/uploads", "./uploads", fiber.Static{
    Compress:      true,
    ByteRange:     true,
    Browse:        false,
    CacheDuration: 24 * time.Hour,
  })

  port := os.Getenv("PORT")
  if port == "" {
    port = ":8080"
  }

  err = f.Listen(port)
  if err != nil {
    log.Fatal("Failed to start server:", err)
  }
}
