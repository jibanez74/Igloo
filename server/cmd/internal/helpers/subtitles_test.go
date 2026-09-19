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

func TestIsBitmapSubtitleCodecIgnoresSurroundingWhitespace(t *testing.T) {
	t.Parallel()

	if !IsBitmapSubtitleCodec(" dvd_subtitle ") {
		t.Error("IsBitmapSubtitleCodec(\" dvd_subtitle \") = false, want true")
	}
}

func TestSubtitleCacheKey(t *testing.T) {
	t.Parallel()

	key := SubtitleCacheKey("movie", 12, 3)
	prefix := SubtitleCachePrefix("movie", 12)

	if !strings.HasPrefix(key, prefix) {
		t.Fatalf("SubtitleCacheKey(...) = %q, want it to start with %q", key, prefix)
	}
	if key == prefix {
		t.Fatal("SubtitleCacheKey(...) did not append the stream index to the prefix")
	}
}

// kind separates the movie and show file id spaces, so the same file id and
// stream index in each must never produce the same cache entry.
func TestSubtitleCacheKeySeparatesMediaKinds(t *testing.T) {
	t.Parallel()

	movieKey := SubtitleCacheKey("movie", 12, 3)
	showKey := SubtitleCacheKey("show", 12, 3)
	if movieKey == showKey {
		t.Fatalf("movie and show cache keys collide: %q", movieKey)
	}

	if strings.HasPrefix(showKey, SubtitleCachePrefix("movie", 12)) {
		t.Errorf("show key %q matches the movie prefix", showKey)
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
