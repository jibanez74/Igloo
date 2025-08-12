package helpers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func SaveAvatar(file *multipart.FileHeader, dir string) (string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		return "", fmt.Errorf("file must have an extension")
	}

	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return "", errors.New("invalid file type. Only .jpg, .jpeg, and .png files are allowed")
	}

	uuid := uuid.New().String()
	fullPath := filepath.Join(dir, uuid+ext)

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return fullPath, nil
}

func SaveTmdbImage(tmdbUrl, output, fileName string) error {
	if tmdbUrl == "" || output == "" || fileName == "" {
		return errors.New("tmdbUrl, output, and fileName are required")
	}

	err := os.MkdirAll(output, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputFile := filepath.Join(output, fileName)

	_, err = os.Stat(outputFile)
	if err == nil {
		return nil
	}

	response, err := http.Get(tmdbUrl)
	if err != nil {
		return fmt.Errorf("failed to get image from tmdb: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download image: status code %d", response.StatusCode)
	}

	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, response.Body)
	if err != nil {
		os.Remove(outputFile) // Clean up if copy fails
		return fmt.Errorf("failed to save image: %w", err)
	}

	return nil
}
