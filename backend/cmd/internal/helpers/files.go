package helpers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var videoExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".mpeg": true,
	".mpg":  true,
	".3gp":  true,
}

func IsVideoFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return videoExtensions[ext]
}

func CheckDirExistAndReadable(dirPath string) error {
	if dirPath == "" {
		return errors.New("directory path is required")
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", dirPath)
		}

		return fmt.Errorf("failed to access directory %s: %v", dirPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dirPath)
	}

	file, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("directory is not readable: %s", dirPath)
	}
	file.Close()

	return nil
}

func GetTitleAndYearFromFileName(fileName string) (*TitleYearResponse, error) {
	baseName := filepath.Base(fileName)
	ext := filepath.Ext(baseName)
	fileNameWithoutExt := strings.TrimSuffix(baseName, ext)

	parts := strings.Split(fileNameWithoutExt, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid filename format: %s", fileName)
	}

	yearStr := parts[len(parts)-1]

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return &TitleYearResponse{
			Title: fileNameWithoutExt,
			Year:  0,
		}, fmt.Errorf("invalid year in filename: %s", fileName)
	}

	titleParts := parts[:len(parts)-1]
	title := strings.Join(titleParts, " ")

	return &TitleYearResponse{
		Title: title,
		Year:  year,
	}, nil
}
