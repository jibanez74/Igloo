package helpers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

func SaveImage(url, dir, fileName string) (*string, error) {
	if url == "" || dir == "" || fileName == "" {
		return nil, errors.New("url, dir, and fileName are required to save an image")
	}

	var dirExist bool

	_, err := os.Stat(dir)
	if err != nil {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		dirExist = false
	} else {
		dirExist = true
	}

	fullPath := filepath.Join(dir, fileName)

	if dirExist {
		_, err := os.Stat(fullPath)
		if err == nil {
			return &fullPath, nil
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		return nil, fmt.Errorf("bad status from TMDB: %s", resp.Status)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to save image: %w", err)
	}

	return &fullPath, nil
}
