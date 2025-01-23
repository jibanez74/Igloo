package helpers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func SaveImage(url string, title string, destination string) (string, error) {
	if !strings.Contains(url, "image.tmdb.org") {
		return "", fmt.Errorf("invalid TMDB image URL")
	}

	if err := os.MkdirAll(destination, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	imageType := "poster"
	if strings.Contains(url, "backdrop") {
		imageType = "backdrop"
	}

	safeTitle := strings.ReplaceAll(strings.ToLower(title), " ", "_")
	safeTitle = strings.ReplaceAll(safeTitle, "'", "")
	safeTitle = strings.ReplaceAll(safeTitle, ":", "")
	filename := fmt.Sprintf("%s_%s.jpg", safeTitle, imageType)

	fullPath := filepath.Join(destination, filename)

	if _, err := os.Stat(fullPath); err == nil {
		// File exists, return existing filename
		return filename, nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status from TMDB: %s", resp.Status)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return filename, nil
}
