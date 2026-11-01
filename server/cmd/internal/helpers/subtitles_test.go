package helpers

import (
	"strings"
	"testing"
)

func TestIsBitmapSubtitleCodec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		codec string
		want  bool
	}{
		{name: "PGS", codec: "hdmv_pgs_subtitle", want: true},
		{name: "DVD", codec: "dvd_subtitle", want: true},
		{name: "DVB case insensitive", codec: "DVB_SUBTITLE", want: true},
		{name: "surrounding whitespace", codec: " dvd_subtitle ", want: true},
		{name: "text subtitle", codec: "subrip", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := IsBitmapSubtitleCodec(tt.codec)
			if got != tt.want {
				t.Fatalf("IsBitmapSubtitleCodec(%q) = %v, want %v", tt.codec, got, tt.want)
			}
		})
	}
}

// The key format is pinned as a literal: kind separates the movie and episode
// file id spaces, and the trailing stream index must be present, or every
// track of one file would share a cache entry.
func TestSubtitleCacheKeyAndPrefix(t *testing.T) {
	t.Parallel()

	key := SubtitleCacheKey("movie", 12, 3)
	if key != "sub:movie:12:3" {
		t.Fatalf("SubtitleCacheKey(movie, 12, 3) = %q, want %q", key, "sub:movie:12:3")
	}

	prefix := SubtitleCachePrefix("movie", 12)
	if prefix != "sub:movie:12:" {
		t.Fatalf("SubtitleCachePrefix(movie, 12) = %q, want %q", prefix, "sub:movie:12:")
	}
}

// A prefix is used to evict every track cached for one file, so it must not
// also match a different file whose id merely starts with the same digits.
func TestSubtitleCachePrefixDoesNotMatchOtherFileIDs(t *testing.T) {
	t.Parallel()

	prefix := SubtitleCachePrefix("movie", 1)
	if strings.HasPrefix(SubtitleCacheKey("movie", 12, 3), prefix) {
		t.Errorf("prefix for file 1 (%q) also matches a key for file 12", prefix)
	}
}

func TestSubtitleMatroskaPlanFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		codec     string
		container string
		want      SubtitleMatroskaPlan
	}{
		{name: "SubRip is copied", codec: "subrip", container: "mkv", want: SubtitleMatroskaCopy},
		{name: "ASS is copied", codec: "ass", container: "mkv", want: SubtitleMatroskaCopy},
		{name: "SSA is copied", codec: "ssa", container: "mkv", want: SubtitleMatroskaCopy},
		{name: "PGS is copied", codec: "hdmv_pgs_subtitle", container: "mkv", want: SubtitleMatroskaCopy},
		{name: "PGS in MP4 is copied", codec: "hdmv_pgs_subtitle", container: "mp4", want: SubtitleMatroskaCopy},
		{name: "DVB is copied", codec: "dvb_subtitle", container: "mkv", want: SubtitleMatroskaCopy},
		{name: "VobSub in Matroska is copied", codec: "dvd_subtitle", container: "mkv", want: SubtitleMatroskaCopy},
		{name: "VobSub elsewhere has no palette", codec: "dvd_subtitle", container: "mp4", want: SubtitleMatroskaUnsupported},
		{name: "case and whitespace", codec: " HDMV_PGS_SUBTITLE ", container: " MKV ", want: SubtitleMatroskaCopy},
		{name: "mov_text is converted", codec: "mov_text", container: "mp4", want: SubtitleMatroskaSubRip},
		{name: "EIA-608 is converted", codec: "eia_608", container: "mp4", want: SubtitleMatroskaSubRip},
		{name: "WebVTT is converted", codec: "webvtt", container: "mkv", want: SubtitleMatroskaSubRip},
		{name: "plain text is converted", codec: "text", container: "mkv", want: SubtitleMatroskaSubRip},
		{name: "XSUB is unsupported", codec: "xsub", container: "avi", want: SubtitleMatroskaUnsupported},
		{name: "teletext is unsupported", codec: "dvb_teletext", container: "mkv", want: SubtitleMatroskaUnsupported},
		{name: "empty codec is unsupported", codec: "", container: "mkv", want: SubtitleMatroskaUnsupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SubtitleMatroskaPlanFor(tt.codec, tt.container)
			if got != tt.want {
				t.Fatalf("SubtitleMatroskaPlanFor(%q, %q) = %v, want %v", tt.codec, tt.container, got, tt.want)
			}
		})
	}
}
