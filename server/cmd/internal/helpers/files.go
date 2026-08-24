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

var AudioMimeTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
}

var ValidVideoExtensions = map[string]bool{
	"mp4":  true,
	"avi":  true,
	"mkv":  true,
	"mov":  true,
	"m4v":  true,
	"webm": true,
}

// VideoMimeTypes pins the container→MIME mapping for movie files. Deriving it
// with mime.TypeByExtension is host-dependent (/etc/mime.types overrides Go's
// table and maps .webm to audio/webm; minimal images have no table at all),
// which made playback eligibility and Content-Type vary by machine — see
// "Direct Play Eligibility and Fallback" in docs/ffmpeg.md. Keys must match
// ValidVideoExtensions exactly.
var VideoMimeTypes = map[string]string{
	"mp4":  "video/mp4",
	"m4v":  "video/mp4",
	"mkv":  "video/x-matroska",
	"webm": "video/webm",
	"avi":  "video/x-msvideo",
	"mov":  "video/quicktime",
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

func isReasonableYear(n int) bool {
	return n >= 1900 && n <= 2100
}

// IsMovieReleaseNoiseToken reports whether token is a movie release or codec marker
// that should not be treated as part of a movie title.
func IsMovieReleaseNoiseToken(token string) bool {
	return movieReleaseNoiseTokens[strings.ToLower(strings.TrimSpace(token))]
}

func GetTitleAndYearFromFileName(fileName string) (*TitleYearResponse, error) {
	baseName := filepath.Base(fileName)
	ext := filepath.Ext(baseName)
	s := strings.TrimSuffix(baseName, ext)
	s = strings.TrimSpace(s)

	if s == "" {
		return nil, fmt.Errorf("empty filename: %s", fileName)
	}

	open := strings.LastIndex(s, "(")
	if open >= 0 {
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
		if err != nil || !isReasonableYear(y) {
			continue
		}
		titleParts := parts[:i]
		title := strings.TrimSpace(strings.Join(titleParts, " "))
		if title != "" {
			return &TitleYearResponse{Title: title, Year: y}, nil
		}
	}

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

	return strings.ToLower(ext[1:])
}
