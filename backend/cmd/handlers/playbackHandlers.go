package handlers

import (
	"crypto/md5"
	"errors"
	"fmt"
	"igloo/cmd/helpers"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *Handlers) PlayVideo(c *fiber.Ctx) error {
	filePath := c.Query("file_path")
	if filePath == "" {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Error": "File path is required",
		})

		return errors.New("file path is required")
	}

	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) || os.IsPermission(err) {
		c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "File not found or not readable",
		})

		return errors.New("file not found or not readable")
	}

	size64 := fileInfo.Size()

	videoContainer := c.Query("container")
	if videoContainer == "" {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No container provided",
		})

		return errors.New("no container provided")
	}

	rangeHeader := c.Get("Range")
	if rangeHeader == "" {
		return c.SendFile(filePath)
	}

	chunkSizeStr := c.Query("chunk_size", "10485760") // Default to 10 MB if not specified

	chunkSize, err := strconv.ParseInt(chunkSizeStr, 10, 64)
	if err != nil {
		chunkSize = int64(10 * 1024 * 1024) // Default to 10 MB
	}

	start, end := helpers.ParseRange(rangeHeader, size64, chunkSize)

	c.Set("Cache-Control", "public, max-age=31536000")
	etag := fmt.Sprintf("%x", md5.Sum([]byte(filePath)))
	c.Set("ETag", etag)

	ifNoneMatch := c.Get("If-None-Match")
	if ifNoneMatch == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}

	contentLength := end - start + 1
	c.Status(fiber.StatusPartialContent)
	c.Set("Content-Type", "video/"+videoContainer)
	c.Set("Accept-Ranges", "bytes")
	c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size64))
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))

	return c.SendFile(filePath, true)
}
