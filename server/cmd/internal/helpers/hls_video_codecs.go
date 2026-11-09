package helpers

import "strings"

// HLSVideoCodecHEVC is the only codec video_codecs accepts today.
const HLSVideoCodecHEVC = "hevc"

// HLSVideoCodecs is the validated video_codecs query value: the codecs a
// client decodes from fragmented MP4 besides H.264, which every client does.
// It only widens what a remux request may copy. The zero value is H.264 only.
type HLSVideoCodecs struct {
	HEVC bool
}

// ParseHLSVideoCodecs validates a video_codecs value, a comma-separated list.
// Like audio_codec it is case-sensitive. An empty list or an unknown codec is
// invalid, so a typo never silently degrades to a transcode.
func ParseHLSVideoCodecs(value string) (HLSVideoCodecs, bool) {
	var codecs HLSVideoCodecs
	for token := range strings.SplitSeq(value, ",") {
		if strings.TrimSpace(token) != HLSVideoCodecHEVC {
			return HLSVideoCodecs{}, false
		}
		codecs.HEVC = true
	}
	return codecs, true
}

// QueryValue is the canonical video_codecs value for rewritten asset URLs, or
// "" when the request named none.
func (c HLSVideoCodecs) QueryValue() string {
	if c.HEVC {
		return HLSVideoCodecHEVC
	}
	return ""
}

// SessionKey is the video part of a personal session identity.
func (c HLSVideoCodecs) SessionKey() string {
	if c.HEVC {
		return "video:hevc"
	}
	return "video:h264"
}

// IsHEVC reports an HEVC video stream by its ffprobe codec name.
func IsHEVC(codec string) bool {
	switch normalizeCodec(codec) {
	case "hevc", "h265", "h.265":
		return true
	default:
		return false
	}
}

// HLSCopyVideoTag is the MP4 sample-entry tag a copied stream is written
// with, or "" for FFmpeg's default. FFmpeg tags copied HEVC hev1, which Safari
// refuses; hvc1 plays everywhere fMP4 HEVC does.
func HLSCopyVideoTag(codec string) string {
	if IsHEVC(codec) {
		return "hvc1"
	}
	return ""
}
