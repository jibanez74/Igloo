package helpers

import "strings"

var coverArtVideoCodecs = map[string]bool{
	"mjpeg": true,
	"png":   true,
	"gif":   true,
	"bmp":   true,
}

// IsCoverArtVideoCodec reports still-image video streams used as embedded cover art.
func IsCoverArtVideoCodec(codec string) bool {
	return coverArtVideoCodecs[strings.ToLower(codec)]
}
