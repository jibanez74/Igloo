package helpers

import (
	"fmt"
	"strings"
)

const subtitleCacheKeyPrefix = "sub:"

var bitmapSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true,
	"dvd_subtitle":      true,
	"dvb_subtitle":      true,
}

// IsBitmapSubtitleCodec returns true for image-based subtitle codecs
// (PGS, DVD sub) that cannot be converted to WebVTT.
func IsBitmapSubtitleCodec(codec string) bool {
	return bitmapSubtitleCodecs[strings.ToLower(codec)]
}

func SubtitleCacheKey(movieID int64, streamIndex int64) string {
	return fmt.Sprintf("%s%d:%d", subtitleCacheKeyPrefix, movieID, streamIndex)
}

func SubtitleCachePrefix(movieID int64) string {
	return fmt.Sprintf("%s%d:", subtitleCacheKeyPrefix, movieID)
}
