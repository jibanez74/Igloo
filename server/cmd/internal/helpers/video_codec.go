package helpers

// coverArtVideoCodecs are the still-image codecs an import treats as embedded
// cover art rather than as a video stream.
var coverArtVideoCodecs = map[string]bool{
	"png": true,
	"gif": true,
	"bmp": true,
}

// playbackCoverArtVideoCodecs mirrors COVER_ART_CODECS in
// web/src/lib/playback.ts so the client and the server never judge a different
// stream to be the feature. It is the import set plus MJPEG, derived from it so
// a codec added there cannot be forgotten here.
var playbackCoverArtVideoCodecs = withPlaybackOnlyCoverArtCodecs(coverArtVideoCodecs)

func withPlaybackOnlyCoverArtCodecs(base map[string]bool) map[string]bool {
	codecs := make(map[string]bool, len(base)+1)
	for codec, isCoverArt := range base {
		codecs[codec] = isCoverArt
	}
	// MJPEG is playback-only: docs/ffmpeg.md keeps moving MJPEG an accepted
	// video codec, so MJPEG alone does not make a stream artwork on import.
	codecs["mjpeg"] = true
	return codecs
}

// IsCoverArtVideoCodec reports still-image video streams used as embedded cover
// art on import. MJPEG is absent on purpose; withPlaybackOnlyCoverArtCodecs
// says why.
func IsCoverArtVideoCodec(codec string) bool {
	return coverArtVideoCodecs[normalizeCodec(codec)]
}

// IsPlaybackCoverArtVideoCodec reports the streams to skip when picking the
// feature video stream to play, which is a wider question than what an import
// rejects as artwork.
func IsPlaybackCoverArtVideoCodec(codec string) bool {
	return playbackCoverArtVideoCodecs[normalizeCodec(codec)]
}
