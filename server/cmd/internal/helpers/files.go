package helpers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type TitleYearResponse struct {
	Title string
	Year  int
}

var ValidAudioExtensions = map[string]bool{
	"mp3":  true,
	"flac": true,
	"m4a":  true,
}

var ValidVideoExtensions = map[string]bool{
	"mp4": true,
	"avi": true,
	"mkv": true,
	"mov": true,
	"m4v": true,
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

func GetFileExtension(path string) string {
	ext := filepath.Ext(path)

	if len(ext) == 0 {
		return ""
	}

	return ext[1:]
}

// CalculateFileHash calculates the SHA256 hash of a file
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to read file for hashing: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
