package main

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

// ---- finalizeEventPlaylist ----

func TestFinalizeEventPlaylist(t *testing.T) {
	t.Run("replaces EVENT with VOD playlist type", func(t *testing.T) {
		input := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n"
		result := finalizeEventPlaylist(input)
		if strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:EVENT") {
			t.Error("Result still contains EVENT playlist type")
		}
		if !strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:VOD") {
			t.Error("Result does not contain VOD playlist type")
		}
	})

	t.Run("appends ENDLIST when missing", func(t *testing.T) {
		input := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n"
		result := finalizeEventPlaylist(input)
		if !strings.Contains(result, "#EXT-X-ENDLIST") {
			t.Error("Result does not contain #EXT-X-ENDLIST")
		}
	})

	t.Run("does not duplicate ENDLIST when already present", func(t *testing.T) {
		input := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
		result := finalizeEventPlaylist(input)
		count := strings.Count(result, "#EXT-X-ENDLIST")
		if count != 1 {
			t.Errorf("Expected exactly 1 #EXT-X-ENDLIST, got %d", count)
		}
	})

	t.Run("preserves other playlist content", func(t *testing.T) {
		input := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n"
		result := finalizeEventPlaylist(input)
		for _, tag := range []string{"#EXTM3U", "#EXT-X-VERSION:7", "segment_0.m4s"} {
			if !strings.Contains(result, tag) {
				t.Errorf("Result is missing expected content %q", tag)
			}
		}
	})

	t.Run("handles already-finalized VOD playlist without duplicating ENDLIST", func(t *testing.T) {
		input := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
		result := finalizeEventPlaylist(input)
		if !strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:VOD") {
			t.Error("Result lost VOD type when input had no EVENT type")
		}
		if strings.Count(result, "#EXT-X-ENDLIST") != 1 {
			t.Error("Expected exactly 1 ENDLIST in already-finalized playlist")
		}
	})

	t.Run("only first EVENT occurrence is replaced", func(t *testing.T) {
		// strings.Replace with n=1 only replaces the first occurrence
		input := "#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-PLAYLIST-TYPE:EVENT\n"
		result := finalizeEventPlaylist(input)
		// Only first replaced
		vodCount := strings.Count(result, "#EXT-X-PLAYLIST-TYPE:VOD")
		eventCount := strings.Count(result, "#EXT-X-PLAYLIST-TYPE:EVENT")
		if vodCount != 1 {
			t.Errorf("Expected 1 VOD tag, got %d", vodCount)
		}
		if eventCount != 1 {
			t.Errorf("Expected 1 remaining EVENT tag, got %d", eventCount)
		}
	})

	t.Run("empty input gets ENDLIST appended", func(t *testing.T) {
		result := finalizeEventPlaylist("")
		if !strings.Contains(result, "#EXT-X-ENDLIST") {
			t.Error("Expected #EXT-X-ENDLIST to be appended to empty input")
		}
	})
}

// ---- rewritePlaylistURLs ----

func TestRewritePlaylistURLs(t *testing.T) {
	t.Run("prepends baseURL to segment filenames", func(t *testing.T) {
		playlist := "#EXTM3U\n#EXTINF:4.0,\nsegment_0.m4s\n#EXTINF:4.0,\nsegment_1.m4s\n#EXT-X-ENDLIST\n"
		baseURL := "/api/movies/1/hls/1080p_4mbps/"
		result := rewritePlaylistURLs(playlist, baseURL, 0)
		if !strings.Contains(result, baseURL+"segment_0.m4s?audio_track=0") {
			t.Errorf("Expected segment_0.m4s to be rewritten, got:\n%s", result)
		}
		if !strings.Contains(result, baseURL+"segment_1.m4s?audio_track=0") {
			t.Errorf("Expected segment_1.m4s to be rewritten, got:\n%s", result)
		}
	})

	t.Run("rewrites EXT-X-MAP URI for init file", func(t *testing.T) {
		playlist := fmt.Sprintf("#EXTM3U\n#EXT-X-MAP:URI=\"%s\"\n#EXTINF:4.0,\nsegment_0.m4s\n",
			helpers.HLS_INIT_FILENAME)
		baseURL := "/api/movies/5/hls/remux/"
		audioTrack := 2
		result := rewritePlaylistURLs(playlist, baseURL, audioTrack)
		expected := fmt.Sprintf(`URI="%s%s?audio_track=%d"`, baseURL, helpers.HLS_INIT_FILENAME, audioTrack)
		if !strings.Contains(result, expected) {
			t.Errorf("Expected EXT-X-MAP URI %q, got:\n%s", expected, result)
		}
	})

	t.Run("appends correct audio_track query param", func(t *testing.T) {
		playlist := "#EXTM3U\n#EXTINF:4.0,\nsegment_0.m4s\n"
		result := rewritePlaylistURLs(playlist, "/base/", 3)
		if !strings.Contains(result, "?audio_track=3") {
			t.Errorf("Expected audio_track=3, got:\n%s", result)
		}
	})

	t.Run("comment lines and HLS tags are passed through unchanged", func(t *testing.T) {
		playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:8\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
		result := rewritePlaylistURLs(playlist, "/base/", 0)
		for _, tag := range []string{"#EXTM3U", "#EXT-X-VERSION:7", "#EXT-X-ENDLIST"} {
			if !strings.Contains(result, tag) {
				t.Errorf("Tag %q was dropped from result", tag)
			}
		}
	})

	t.Run("audioTrack=0 produces ?audio_track=0 suffix", func(t *testing.T) {
		playlist := "#EXTINF:4.0,\nsegment_0.m4s\n"
		result := rewritePlaylistURLs(playlist, "/b/", 0)
		if !strings.Contains(result, "?audio_track=0") {
			t.Errorf("Expected audio_track=0, got: %s", result)
		}
	})

	t.Run("segment lines are not treated as comment lines", func(t *testing.T) {
		playlist := "#EXTINF:4.0,\nsegment_0.m4s\n"
		result := rewritePlaylistURLs(playlist, "/x/", 1)
		// segment_0.m4s should be prefixed, not left bare
		if strings.Contains(result, "\nsegment_0.m4s") {
			t.Errorf("Segment line was not rewritten: %s", result)
		}
		if !strings.Contains(result, "/x/segment_0.m4s?audio_track=1") {
			t.Errorf("Expected rewritten segment URL, got: %s", result)
		}
	})

	t.Run("empty playlist returns empty-ish string", func(t *testing.T) {
		result := rewritePlaylistURLs("", "/base/", 0)
		// Should not panic; result is a single newline from the split+write loop
		_ = result
	})
}

// ---- generateVODPlaylist ----

func TestGenerateVODPlaylist(t *testing.T) {
	t.Run("contains required M3U8 headers", func(t *testing.T) {
		result := generateVODPlaylist(120.0, "/base/", 0)
		for _, required := range []string{
			"#EXTM3U",
			"#EXT-X-VERSION:7",
			"#EXT-X-TARGETDURATION:",
			"#EXT-X-MEDIA-SEQUENCE:0",
			"#EXT-X-PLAYLIST-TYPE:VOD",
			"#EXT-X-ENDLIST",
		} {
			if !strings.Contains(result, required) {
				t.Errorf("Missing required tag %q", required)
			}
		}
	})

	t.Run("contains EXT-X-MAP for init file with correct base URL", func(t *testing.T) {
		baseURL := "/api/movies/1/hls/1080p_8mbps/"
		result := generateVODPlaylist(60.0, baseURL, 0)
		expected := fmt.Sprintf(`#EXT-X-MAP:URI="%s%s?audio_track=0"`, baseURL, helpers.HLS_INIT_FILENAME)
		if !strings.Contains(result, expected) {
			t.Errorf("Expected EXT-X-MAP line %q not found in:\n%s", expected, result)
		}
	})

	t.Run("correct segment count for exact multiple of segment duration", func(t *testing.T) {
		segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
		totalDur := segDur * 10
		result := generateVODPlaylist(totalDur, "/b/", 0)
		segCount := strings.Count(result, helpers.HLS_SEGMENT_FILENAME_PREFIX)
		// EXT-X-MAP also contains the baseURL but not the segment prefix; count segment_ occurrences
		if segCount != 10 {
			t.Errorf("Expected 10 segments for %v sec duration, got %d", totalDur, segCount)
		}
	})

	t.Run("correct segment count when duration is not exact multiple", func(t *testing.T) {
		segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
		totalDur := segDur*7 + 1.5 // 7 full + 1 partial = 8 segments
		expected := int(math.Ceil(totalDur / segDur))
		result := generateVODPlaylist(totalDur, "/b/", 0)
		segCount := strings.Count(result, helpers.HLS_SEGMENT_FILENAME_PREFIX)
		if segCount != expected {
			t.Errorf("Expected %d segments, got %d", expected, segCount)
		}
	})

	t.Run("segments are named with correct prefix and suffix", func(t *testing.T) {
		result := generateVODPlaylist(float64(helpers.HLS_SEGMENT_TIME_SEC), "/b/", 0)
		if !strings.Contains(result, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX) {
			t.Error("segment_0.m4s not found in playlist")
		}
	})

	t.Run("last segment duration is clipped to remaining time", func(t *testing.T) {
		segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
		remainder := 1.5
		totalDur := segDur*3 + remainder
		result := generateVODPlaylist(totalDur, "/b/", 0)
		expected := fmt.Sprintf("#EXTINF:%.6f,", remainder)
		if !strings.Contains(result, expected) {
			t.Errorf("Expected last segment duration %s, not found in:\n%s", expected, result)
		}
	})

	t.Run("single segment for very short duration", func(t *testing.T) {
		result := generateVODPlaylist(0.001, "/b/", 0)
		segCount := strings.Count(result, helpers.HLS_SEGMENT_FILENAME_PREFIX)
		if segCount < 1 {
			t.Error("Expected at least 1 segment for tiny duration")
		}
	})

	t.Run("audio track query param added to all segment and init URIs", func(t *testing.T) {
		result := generateVODPlaylist(float64(helpers.HLS_SEGMENT_TIME_SEC)*2, "/b/", 2)
		for _, line := range strings.Split(result, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "/b/"+helpers.HLS_SEGMENT_FILENAME_PREFIX) {
				if !strings.Contains(trimmed, "audio_track=2") {
					t.Errorf("Expected audio_track=2 in segment line: %q", trimmed)
				}
			}
			if strings.HasPrefix(trimmed, "#EXT-X-MAP:") && strings.Contains(trimmed, helpers.HLS_INIT_FILENAME) {
				if !strings.Contains(trimmed, "audio_track=2") {
					t.Errorf("Expected audio_track=2 in EXT-X-MAP line: %q", trimmed)
				}
			}
		}
	})

	t.Run("target duration is double the segment time", func(t *testing.T) {
		result := generateVODPlaylist(60.0, "/b/", 0)
		expectedTarget := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", helpers.HLS_SEGMENT_TIME_SEC*2)
		if !strings.Contains(result, expectedTarget) {
			t.Errorf("Expected TARGETDURATION=%d, not found in:\n%s", helpers.HLS_SEGMENT_TIME_SEC*2, result)
		}
	})

	t.Run("zero duration produces at least one segment", func(t *testing.T) {
		result := generateVODPlaylist(0.0, "/b/", 0)
		segCount := strings.Count(result, helpers.HLS_SEGMENT_FILENAME_PREFIX)
		if segCount < 1 {
			t.Error("Expected at least 1 segment for zero duration input")
		}
	})

	t.Run("playlist ends with ENDLIST tag", func(t *testing.T) {
		result := generateVODPlaylist(30.0, "/b/", 0)
		trimmed := strings.TrimRight(result, "\n")
		if !strings.HasSuffix(trimmed, "#EXT-X-ENDLIST") {
			offset := len(trimmed) - 40
			if offset < 0 {
				offset = 0
			}
			t.Errorf("Expected playlist to end with #EXT-X-ENDLIST, got:\n...%s", trimmed[offset:])
		}
	})
}