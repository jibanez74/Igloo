package helpers

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type TitleYear struct {
	Title string
	Year  int
}

var movieReleaseNoiseTokens = map[string]bool{
	"1080p": true, "720p": true, "480p": true, "2160p": true, "4k": true,
	"bluray": true, "brrip": true, "webrip": true, "web": true, "web-dl": true, "webdl": true,
	"dvdrip": true, "hdrip": true, "remux": true, "repack": true, "proper": true, "remastered": true,
	"extended": true,
	"h264":     true, "h265": true, "x264": true, "x265": true, "hevc": true, "av1": true,
	"10bit": true, "8bit": true, "hdr": true, "sdr": true,
	"aac": true, "aac5": true, "aac51": true, "ddp": true, "ac3": true, "dts": true,
	"dtshd": true, "atmos": true, "truehd": true,
	"mkv": true, "mp4": true,
	"yts": true, "ytsmx": true, "mx": true,
}

// IsReasonableYear reports whether n is a plausible movie release year, used to
// tell a year token in a filename apart from any other four-digit number.
func IsReasonableYear(n int) bool {
	return n >= 1900 && n <= 2100
}

// IsMovieReleaseNoiseToken reports whether token is a movie release or codec marker
// that should not be treated as part of a movie title.
func IsMovieReleaseNoiseToken(token string) bool {
	return movieReleaseNoiseTokens[strings.ToLower(strings.TrimSpace(token))]
}

func TitleAndYearFromFileName(fileName string) (TitleYear, error) {
	baseName := filepath.Base(fileName)
	ext := filepath.Ext(baseName)
	s := strings.TrimSuffix(baseName, ext)
	s = strings.TrimSpace(s)

	if s == "" {
		return TitleYear{}, fmt.Errorf("empty filename: %s", fileName)
	}

	open := strings.LastIndex(s, "(")
	if open >= 0 {
		if close := strings.Index(s[open:], ")"); close >= 0 {
			close += open
			yearStr := strings.TrimSpace(s[open+1 : close])
			if len(yearStr) == 4 {
				if y, err := strconv.Atoi(yearStr); err == nil && IsReasonableYear(y) {
					title := strings.TrimSpace(s[:open])
					title = strings.ReplaceAll(title, ".", " ")
					if title != "" {
						return TitleYear{Title: title, Year: y}, nil
					}
				}
			}
		}
	}

	parts := strings.Split(s, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		tok := strings.TrimSpace(parts[i])
		if IsMovieReleaseNoiseToken(tok) {
			continue
		}
		if len(tok) != 4 {
			continue
		}
		y, err := strconv.Atoi(tok)
		if err != nil || !IsReasonableYear(y) {
			continue
		}
		titleParts := parts[:i]
		title := strings.TrimSpace(strings.Join(titleParts, " "))
		if title != "" {
			return TitleYear{Title: title, Year: y}, nil
		}
	}

	words := strings.Fields(s)
	if len(words) >= 2 {
		last := words[len(words)-1]
		if len(last) == 4 {
			if y, err := strconv.Atoi(last); err == nil && IsReasonableYear(y) {
				title := strings.TrimSpace(strings.Join(words[:len(words)-1], " "))
				if title != "" {
					return TitleYear{Title: title, Year: y}, nil
				}
			}
		}
	}

	title := strings.ReplaceAll(s, ".", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		title = s
	}
	return TitleYear{Title: title, Year: 0}, nil
}

func FileExtension(path string) string {
	ext := filepath.Ext(path)

	if len(ext) == 0 {
		return ""
	}

	return strings.ToLower(ext[1:])
}
