package helpers

import (
	"fmt"
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
	"aac":  true,
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

func IsVideoFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return false
	}

	extWithoutDot := strings.TrimPrefix(ext, ".")

	return ValidVideoExtensions[extWithoutDot]
}

func IsTrackFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return false
	}

	extWithoutDot := strings.TrimPrefix(ext, ".")

	return ValidAudioExtensions[extWithoutDot]
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
