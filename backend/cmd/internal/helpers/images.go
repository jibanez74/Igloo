package helpers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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
