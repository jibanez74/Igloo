package main

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (app *application) getTotalMovieCount(c *fiber.Ctx) error {
	count, err := app.queries.GetMovieCount(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"count": count,
	})
}

func (app *application) getLatestMovies(c *fiber.Ctx) error {
	movies, err := app.queries.GetLatestMovies(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movies": movies,
	})
}

func (app *application) getMovieDetails(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to parse id %s", c.Params("id")),
		})
	}

	movie, err := app.queries.GetMovieDetails(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"movie": movie,
	})
}

func (app *application) getMoviesPaginated(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "24"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 24
	}

	offset := (page - 1) * limit

	movies, err := app.queries.GetMoviesPaginated(c.Context(), database.GetMoviesPaginatedParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	count, err := app.queries.GetMovieCount(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": serverErr,
		})
	}

	totalPages := (count + int64(limit) - 1) / int64(limit)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"items":        movies,
		"current_page": page,
		"total_pages":  totalPages,
		"count":        count,
	})
}

func (app *application) streamMovie(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to parse movie id from url params: %v", err),
		})
	}

	movie, err := app.queries.GetMovieForDirectPlayback(c.Context(), int32(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to find movie with id %d: %v", id, err),
		})
	}

	// Ensure the file exists and get its info
	fileInfo, err := os.Stat(movie.FilePath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("movie file not found: %v", err),
		})
	}

	// Log the file details for debugging
	fmt.Printf("Streaming file: %s\n", movie.FilePath)
	fmt.Printf("File size: %d bytes\n", fileInfo.Size())
	fmt.Printf("Content type: %s\n", movie.ContentType)

	// Set headers
	c.Set("Content-Type", movie.ContentType)
	c.Set("Accept-Ranges", "bytes")
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	c.Set("Connection", "close")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Content-Type-Options", "nosniff")

	rangeHdr := c.Get("Range")
	fmt.Printf("Range header: %s\n", rangeHdr)

	if rangeHdr != "" {
		start, end, err := helpers.ParseRange(rangeHdr, fileInfo.Size())
		if err != nil {
			return c.Status(fiber.StatusRequestedRangeNotSatisfiable).JSON(fiber.Map{
				"error": fmt.Sprintf("invalid range request: %v", err),
			})
		}

		// Limit the range to a reasonable size (e.g., 10MB chunks)
		maxChunkSize := int64(10 * 1024 * 1024) // 10MB
		if end-start+1 > maxChunkSize {
			end = start + maxChunkSize - 1
		}

		if end >= fileInfo.Size() {
			end = fileInfo.Size() - 1
		}

		length := end - start + 1
		fmt.Printf("Range request: %d-%d (length: %d)\n", start, end, length)

		file, err := os.Open(movie.FilePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("unable to open movie file: %v", err),
			})
		}
		defer file.Close()

		// Seek to the start position
		_, err = file.Seek(start, io.SeekStart)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to seek in file: %v", err),
			})
		}

		c.Status(fiber.StatusPartialContent)
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileInfo.Size()))
		c.Set("Content-Length", strconv.FormatInt(length, 10))

		// Log the response headers
		fmt.Printf("Response headers:\n")
		fmt.Printf("  Content-Type: %s\n", c.Get("Content-Type"))
		fmt.Printf("  Content-Range: %s\n", c.Get("Content-Range"))
		fmt.Printf("  Content-Length: %s\n", c.Get("Content-Length"))

		// Use a buffer to read the file in chunks
		bufferSize := 1024 * 1024 // 1MB buffer
		return c.SendStream(file, int(length), bufferSize)
	}

	// For full file requests, we'll still use chunked transfer
	c.Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

	// Log the response headers for full file request
	fmt.Printf("Full file response headers:\n")
	fmt.Printf("  Content-Type: %s\n", c.Get("Content-Type"))
	fmt.Printf("  Content-Length: %s\n", c.Get("Content-Length"))

	file, err := os.Open(movie.FilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("unable to open movie file: %v", err),
		})
	}
	defer file.Close()

	// Use a buffer to read the file in chunks
	bufferSize := 1024 * 1024 // 1MB buffer
	return c.SendStream(file, int(fileInfo.Size()), bufferSize)
}
