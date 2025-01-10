package main

import (
  "igloo/cmd/database/models"
  "os"
  "path/filepath"
  "strconv"
  "strings"

  "github.com/gofiber/fiber/v2"
)

func (app *config) ServeHlsMaster(c *fiber.Ctx) error {
  movieID := c.Params("id")
  if movieID == "" {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": "Movie ID is required",
    })
  }

  id, err := strconv.ParseUint(movieID, 10, 32)
  if err != nil {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": "Invalid movie ID",
    })
  }

  var movie models.Movie
  movie.ID = uint(id)

  status, err := app.Repo.GetMovieByID(&movie)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": "Movie not found",
    })
  }

  playlistPath := filepath.Join(app.Settings.TranscodeDir, movieID, "hls", "master.m3u8")

  _, err = os.Stat(playlistPath)
  if os.IsNotExist(err) {
    return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
      "error": "HLS stream not found",
    })
  }

  c.Set("Content-Type", "application/vnd.apple.mpegurl")
  c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
  c.Set("Pragma", "no-cache")
  c.Set("Expires", "0")

  return c.SendFile(playlistPath)
}

func (app *config) ServeHlsSegment(c *fiber.Ctx) error {
  movieID := c.Params("id")
  segment := c.Params("segment")

  if movieID == "" || segment == "" {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": "Invalid request parameters",
    })
  }

  // Convert movieID to uint
  id, err := strconv.ParseUint(movieID, 10, 32)
  if err != nil {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": "Invalid movie ID",
    })
  }

  // Validate segment filename (should only be .ts files)
  if !strings.HasSuffix(segment, ".ts") {
    return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
      "error": "Invalid segment request",
    })
  }

  // Get movie to verify existence and permissions
  var movie models.Movie
  movie.ID = uint(id)
  status, err := app.Repo.GetMovieByID(&movie)
  if err != nil {
    return c.Status(status).JSON(fiber.Map{
      "error": "Movie not found",
    })
  }

  // Construct path to segment
  segmentPath := filepath.Join(app.Settings.TranscodeDir, movieID, "hls", segment)

  // Verify file exists and is within the correct directory
  if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
    return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
      "error": "Segment not found",
    })
  }

  // Set correct content type for MPEG-TS
  c.Set("Content-Type", "video/MP2T")
  // Allow caching of segments
  c.Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours

  return c.SendFile(segmentPath)
}
