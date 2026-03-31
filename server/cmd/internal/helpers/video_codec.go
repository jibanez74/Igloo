package helpers

import "strings"

// IsCoverArtVideoCodec reports still-image video streams used as embedded cover art.
func IsCoverArtVideoCodec(codec string) bool {
	return CoverArtVideoCodecs[strings.ToLower(codec)]
}
