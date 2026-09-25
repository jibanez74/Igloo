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
