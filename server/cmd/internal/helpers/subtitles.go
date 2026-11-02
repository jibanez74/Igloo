package helpers

import "fmt"

const subtitleCacheKeyPrefix = "sub:"

var bitmapSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true,
	"dvd_subtitle":      true,
	"dvb_subtitle":      true,
	"xsub":              true,
}

// IsBitmapSubtitleCodec returns true for image-based subtitle codecs
// (PGS, DVD, DVB, XSUB) that cannot be converted to WebVTT.
func IsBitmapSubtitleCodec(codec string) bool {
	return bitmapSubtitleCodecs[normalizeCodec(codec)]
}

// SubtitleMatroskaPlan says how one subtitle stream travels in a single-track
// Matroska stream.
type SubtitleMatroskaPlan int

const (
	SubtitleMatroskaUnsupported SubtitleMatroskaPlan = iota
	SubtitleMatroskaCopy
	SubtitleMatroskaSubRip
)

// Matroska carries these codecs as they are.
var matroskaCopySubtitleCodecs = map[string]bool{
	"subrip":            true,
	"ass":               true,
	"ssa":               true,
	"hdmv_pgs_subtitle": true,
	"dvd_subtitle":      true,
	"dvb_subtitle":      true,
}

// Data codecs that FFmpeg cannot convert to SubRip.
var unconvertibleSubtitleCodecs = map[string]bool{
	"dvb_teletext": true,
	"arib_caption": true,
}

// SubtitleMatroskaPlanFor copies the codecs Matroska carries and converts
// every other text codec (mov_text, eia_608, webvtt, …) to SubRip. A VobSub
// track is copied only out of Matroska, which keeps its palette in the track's
// codec data; a player cannot draw it without one.
func SubtitleMatroskaPlanFor(codec string, container string) SubtitleMatroskaPlan {
	name := normalizeCodec(codec)
	isVobSubOutsideMatroska := name == "dvd_subtitle" && normalizeCodec(container) != "mkv"
	if isVobSubOutsideMatroska {
		return SubtitleMatroskaUnsupported
	}
	if matroskaCopySubtitleCodecs[name] {
		return SubtitleMatroskaCopy
	}
	isImageBased := bitmapSubtitleCodecs[name] || unconvertibleSubtitleCodecs[name]
	if isImageBased || name == "" {
		return SubtitleMatroskaUnsupported
	}
	return SubtitleMatroskaSubRip
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
