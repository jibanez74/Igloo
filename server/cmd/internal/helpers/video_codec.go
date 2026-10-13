package helpers

import "maps"

// coverArtVideoCodecs are the still-image codecs an import treats as embedded
// cover art rather than as a video stream.
var coverArtVideoCodecs = map[string]bool{
	"png": true,
	"gif": true,
	"bmp": true,
}

// playbackCoverArtVideoCodecs mirrors COVER_ART_CODECS in
// web/src/lib/playback.ts so the client and the server never judge a different
// stream to be the feature: the import set plus MJPEG. MJPEG is playback-only
// because docs/ffmpeg.md keeps moving MJPEG an accepted video codec on import.
var playbackCoverArtVideoCodecs = func() map[string]bool {
	codecs := maps.Clone(coverArtVideoCodecs)
	codecs["mjpeg"] = true
	return codecs
}()

// IsCoverArtVideoCodec reports still-image video streams used as embedded cover
// art on import; MJPEG is absent on purpose (see playbackCoverArtVideoCodecs).
func IsCoverArtVideoCodec(codec string) bool {
	return coverArtVideoCodecs[normalizeCodec(codec)]
}

// IsPlaybackCoverArtVideoCodec reports the streams to skip when picking the
// feature video stream to play, which is a wider question than what an import
// rejects as artwork.
func IsPlaybackCoverArtVideoCodec(codec string) bool {
	return playbackCoverArtVideoCodecs[normalizeCodec(codec)]
}
