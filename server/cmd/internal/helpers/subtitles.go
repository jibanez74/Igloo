package helpers

import (
	"fmt"
	"strings"
)

const subtitleCacheKeyPrefix = "sub:"

// IsBitmapSubtitleCodec returns true for image-based subtitle codecs
// (PGS, DVD sub) that cannot be converted to WebVTT.
func IsBitmapSubtitleCodec(codec string) bool {
	return BitmapSubtitleCodecs[strings.ToLower(codec)]
}

func SubtitleCacheKey(movieID int64, streamIndex int64) string {
	return fmt.Sprintf("%s%d:%d", subtitleCacheKeyPrefix, movieID, streamIndex)
}

func SubtitleCachePrefix(movieID int64) string {
	return fmt.Sprintf("%s%d:", subtitleCacheKeyPrefix, movieID)
}
