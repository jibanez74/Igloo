package main

import (
	"fmt"
	"igloo/cmd/database/models"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (app *config) DirectStreamVideo(c *fiber.Ctx) error {
	log.Println("inside of handler")

	movieID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var movie models.Movie
	movie.ID = uint(movieID)

	status, err := app.repo.GetMovieByID(&movie)
	if err != nil {
		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	fileInfo, err := os.Stat(movie.FilePath)
	if err != nil || os.IsNotExist(err) {
		log.Println(err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	rangeHeader := c.Get("Range")

	c.Set("Content-Type", movie.ContentType)
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileInfo.Name()))

	size := int64(movie.Size)

	var start int64 = 0
	var end int64 = size - 1

	if rangeHeader != "" {
		file, err := os.Open(movie.FilePath)
		if err != nil {
			log.Println(err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		defer file.Close()

		rangeHeader = strings.Replace(rangeHeader, "bytes=", "", 1)
		parts := strings.Split(rangeHeader, "-")

		if parts[0] != "" {
			start, err = strconv.ParseInt(parts[0], 10, 64)
			if err != nil || start < 0 || start >= size {
				start = 0
			}
		}

		if parts[1] != "" {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil || end >= size {
				end = size - 1
			}
		}

		if start > end {
			log.Println("invalid range")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid range",
			})
		}

		contentLength := end - start + 1

		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		c.Set("Accept-Ranges", "bytes")
		c.Set("Content-Length", fmt.Sprintf("%d", contentLength))
		c.Status(fiber.StatusPartialContent)

		_, err = file.Seek(start, 0)
		if err != nil {
			log.Println(err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.SendStream(file)
	}

	c.Set("Content-Length", fmt.Sprintf("%d", size))

	return c.SendFile(movie.FilePath)
}
