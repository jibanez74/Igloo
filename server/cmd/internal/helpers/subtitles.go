package helpers

import "strings"

// IsBitmapSubtitleCodec returns true for image-based subtitle codecs
// (PGS, DVD sub) that cannot be converted to WebVTT.
func IsBitmapSubtitleCodec(codec string) bool {
	return BitmapSubtitleCodecs[strings.ToLower(codec)]
}

// IsTextSubtitleCodec returns true for subtitle codecs that can be
// extracted to WebVTT via ffmpeg.
func IsTextSubtitleCodec(codec string) bool {
	c := strings.ToLower(codec)
	return !BitmapSubtitleCodecs[c] && c != ""
}
