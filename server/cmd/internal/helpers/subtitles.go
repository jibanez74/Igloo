package helpers

import (
	"fmt"
	"strings"
)

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

func SubtitleCacheKey(movieID int64, streamIndex int64) string {
	return fmt.Sprintf("%s%d:%d", SUBTITLE_CACHE_KEY_PREFIX, movieID, streamIndex)
}

func SubtitleCachePrefix(movieID int64) string {
	return fmt.Sprintf("%s%d:", SUBTITLE_CACHE_KEY_PREFIX, movieID)
}
