package helpers

import "fmt"

const subtitleCacheKeyPrefix = "sub:"

var bitmapSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true,
	"dvd_subtitle":      true,
	"dvb_subtitle":      true,
}

// IsBitmapSubtitleCodec returns true for image-based subtitle codecs
// (PGS, DVD sub) that cannot be converted to WebVTT.
func IsBitmapSubtitleCodec(codec string) bool {
	return bitmapSubtitleCodecs[normalizeCodec(codec)]
}

// SubtitleCacheKey names one extracted track. kind separates the movie and
// show file id spaces; fileID is the physical file (a movie id, or a show file
// id shared by every episode in that file), since the VTT depends only on the
// file's bytes.
func SubtitleCacheKey(kind string, fileID int64, streamIndex int64) string {
	return fmt.Sprintf("%s%d", SubtitleCachePrefix(kind, fileID), streamIndex)
}

func SubtitleCachePrefix(kind string, fileID int64) string {
	return fmt.Sprintf("%s%s:%d:", subtitleCacheKeyPrefix, kind, fileID)
}
