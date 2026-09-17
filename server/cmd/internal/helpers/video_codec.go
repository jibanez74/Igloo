package helpers

import "strings"

var coverArtVideoCodecs = map[string]bool{
	"png": true,
	"gif": true,
	"bmp": true,
}

// playbackCoverArtVideoCodecs mirrors COVER_ART_CODECS in
// web/src/lib/playback.ts so the client and the server never judge a different
// stream to be the feature. It is deliberately wider than coverArtVideoCodecs.
var playbackCoverArtVideoCodecs = map[string]bool{
	"mjpeg": true,
	"png":   true,
	"gif":   true,
	"bmp":   true,
}

// IsCoverArtVideoCodec reports still-image video streams used as embedded cover
// art on import. MJPEG is absent on purpose: docs/ffmpeg.md keeps moving MJPEG
// an accepted video codec, so MJPEG alone does not imply artwork.
func IsCoverArtVideoCodec(codec string) bool {
	return coverArtVideoCodecs[strings.ToLower(codec)]
}

// IsPlaybackCoverArtVideoCodec reports the streams to skip when picking the
// feature video stream to play, which is a wider question than what an import
// rejects as artwork.
func IsPlaybackCoverArtVideoCodec(codec string) bool {
	return playbackCoverArtVideoCodecs[strings.ToLower(strings.TrimSpace(codec))]
}
