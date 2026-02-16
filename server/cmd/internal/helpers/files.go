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

// knownNonYearTokens are common dot-separated suffixes that are not a release year.
var knownNonYearTokens = map[string]bool{
	"1080p": true, "720p": true, "480p": true, "2160p": true, "4k": true,
	"bluray": true, "webrip": true, "web-dl": true, "webdl": true,
	"h264": true, "h265": true, "x264": true, "x265": true, "hevc": true,
	"aac": true, "ac3": true, "dts": true, "mkv": true, "mp4": true,
	"remux": true, "repack": true, "proper": true, "extended": true,
}

func isReasonableYear(n int) bool {
	return n >= 1900 && n <= 2100
}

// GetTitleAndYearFromFileName parses a movie filename into title and year.
// Supports: "Title.Word.2020.mkv", "Title.Name.2020.1080p.mkv", "Title (2020).mkv",
// "Title Name 2020.mkv", and similar. Returns year 0 when no year can be parsed.
func GetTitleAndYearFromFileName(fileName string) (*TitleYearResponse, error) {
	baseName := filepath.Base(fileName)
	ext := filepath.Ext(baseName)
	s := strings.TrimSuffix(baseName, ext)
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty filename: %s", fileName)
	}

	// Pattern 1: "Something (2020)" or "Something (2020) Extra"
	if open := strings.LastIndex(s, "("); open >= 0 {
		if close := strings.Index(s[open:], ")"); close >= 0 {
			close += open
			yearStr := strings.TrimSpace(s[open+1 : close])
			if len(yearStr) == 4 {
				if y, err := strconv.Atoi(yearStr); err == nil && isReasonableYear(y) {
					title := strings.TrimSpace(s[:open])
					title = strings.ReplaceAll(title, ".", " ")
					if title != "" {
						return &TitleYearResponse{Title: title, Year: y}, nil
					}
				}
			}
		}
	}

	// Pattern 2: dot-separated — find rightmost segment that is a 4-digit year (skip 1080p, etc.)
	parts := strings.Split(s, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		tok := strings.TrimSpace(parts[i])
		if knownNonYearTokens[strings.ToLower(tok)] {
			continue
		}
		if len(tok) != 4 {
			continue
		}
		y, err := strconv.Atoi(tok)
		if err != nil || !isReasonableYear(y) {
			continue
		}
		titleParts := parts[:i]
		title := strings.TrimSpace(strings.Join(titleParts, " "))
		if title != "" {
			return &TitleYearResponse{Title: title, Year: y}, nil
		}
	}

	// Pattern 3: space-separated — last token is 4-digit year
	words := strings.Fields(s)
	if len(words) >= 2 {
		last := words[len(words)-1]
		if len(last) == 4 {
			if y, err := strconv.Atoi(last); err == nil && isReasonableYear(y) {
				title := strings.TrimSpace(strings.Join(words[:len(words)-1], " "))
				if title != "" {
					return &TitleYearResponse{Title: title, Year: y}, nil
				}
			}
		}
	}

	// Fallback: entire name as title, no year
	title := strings.ReplaceAll(s, ".", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		title = s
	}
	return &TitleYearResponse{Title: title, Year: 0}, nil
}

func GetFileExtension(path string) string {
	ext := filepath.Ext(path)

	if len(ext) == 0 {
		return ""
	}

	return ext[1:]
}
