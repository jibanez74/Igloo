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
	"flac": true,
	"m4a":  true,
}

// AudioMimeTypes maps audio file extensions to their MIME types.
var AudioMimeTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
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
