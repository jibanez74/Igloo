package main

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

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
		if !strings.Contains(result, "#EXTM3U") {
			t.Error("Result is missing #EXTM3U header")
		}
		if !strings.Contains(result, "#EXT-X-VERSION:7") {
			t.Error("Result is missing version tag")
		}
		if !strings.Contains(result, "segment_0.m4s") {
			t.Error("Result is missing segment reference")
		}
	})

	t.Run("handles input without EVENT type gracefully", func(t *testing.T) {
		input := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
		result := finalizeEventPlaylist(input)
		if !strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:VOD") {
			t.Error("Result lost VOD type when input had no EVENT type")
		}
		if strings.Count(result, "#EXT-X-ENDLIST") != 1 {
			t.Error("Expected exactly 1 ENDLIST in already-finalized playlist")
		}
	})
}

func TestRewritePlaylistURLs(t *testing.T) {
	t.Run("prepends baseURL to segment filenames", func(t *testing.T) {
		playlist := "#EXTM3U\n#EXTINF:4.0,\nsegment_0.m4s\n#EXTINF:4.0,\nsegment_1.m4s\n#EXT-X-ENDLIST\n"
		baseURL := "/api/movies/1/hls/1080p_4mbps/"
		result := rewritePlaylistURLs(playlist, baseURL, 0)
		if !strings.Contains(result, baseURL+"segment_0.m4s?audio_track=0") {
			t.Errorf("Expected %s to contain rewritten segment_0.m4s, got:\n%s", "result", result)
		}
		if !strings.Contains(result, baseURL+"segment_1.m4s?audio_track=0") {
			t.Errorf("Expected result to contain rewritten segment_1.m4s")
		}
	})

	t.Run("rewrites EXT-X-MAP URI for init file", func(t *testing.T) {
		playlist := fmt.Sprintf("#EXTM3U\n#EXT-X-MAP:URI=\"%s\"\n#EXTINF:4.0,\nsegment_0.m4s\n", helpers.HLS_INIT_FILENAME)
		baseURL := "/api/movies/5/hls/remux/"
		audioTrack := 2
		result := rewritePlaylistURLs(playlist, baseURL, audioTrack)
		expected := fmt.Sprintf(`URI="%s%s?audio_track=%d"`, baseURL, helpers.HLS_INIT_FILENAME, audioTrack)
		if !strings.Contains(result, expected) {
			t.Errorf("Expected EXT-X-MAP URI to be rewritten to %q, got:\n%s", expected, result)
		}
	})

	t.Run("appends correct audio_track query param", func(t *testing.T) {
		playlist := "#EXTM3U\n#EXTINF:4.0,\nsegment_0.m4s\n"
		result := rewritePlaylistURLs(playlist, "/base/", 3)
		if !strings.Contains(result, "?audio_track=3") {
			t.Errorf("Expected audio_track=3 in result, got:\n%s", result)
		}
	})

	t.Run("comment lines and tags are passed through unchanged", func(t *testing.T) {
		playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:8\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
		result := rewritePlaylistURLs(playlist, "/base/", 0)
		if !strings.Contains(result, "#EXTM3U") {
			t.Error("EXTM3U header was dropped")
		}
		if !strings.Contains(result, "#EXT-X-VERSION:7") {
			t.Error("VERSION tag was dropped")
		}
		if !strings.Contains(result, "#EXT-X-ENDLIST") {
			t.Error("ENDLIST tag was dropped")
		}
	})

	t.Run("audioTrack=0 produces ?audio_track=0 suffix", func(t *testing.T) {
		playlist := "#EXTINF:4.0,\nsegment_0.m4s\n"
		result := rewritePlaylistURLs(playlist, "/b/", 0)
		if !strings.Contains(result, "?audio_track=0") {
			t.Errorf("Expected audio_track=0, got: %s", result)
		}
	})
}

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

	t.Run("contains EXT-X-MAP for init file", func(t *testing.T) {
		baseURL := "/api/movies/1/hls/1080p_8mbps/"
		result := generateVODPlaylist(60.0, baseURL, 0)
		expected := fmt.Sprintf(`#EXT-X-MAP:URI="%s%s?audio_track=0"`, baseURL, helpers.HLS_INIT_FILENAME)
		if !strings.Contains(result, expected) {
			t.Errorf("Expected EXT-X-MAP line %q not found in:\n%s", expected, result)
		}
	})

	t.Run("correct segment count for exact multiple of segment duration", func(t *testing.T) {
		segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
		totalDur := segDur * 10 // exactly 10 segments
		result := generateVODPlaylist(totalDur, "/b/", 0)
		segCount := strings.Count(result, helpers.HLS_SEGMENT_FILENAME_PREFIX)
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
		result := generateVODPlaylist(8.0, "/b/", 0)
		if !strings.Contains(result, helpers.HLS_SEGMENT_FILENAME_PREFIX+"0"+helpers.HLS_SEGMENT_FILENAME_SUFFIX) {
			t.Errorf("segment_0.m4s not found in playlist")
		}
	})

	t.Run("last segment duration is clipped to remaining time", func(t *testing.T) {
		segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
		remainder := 1.5
		totalDur := segDur*3 + remainder
		result := generateVODPlaylist(totalDur, "/b/", 0)
		// The last EXTINF should be remainder (1.5 seconds)
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
		result := generateVODPlaylist(8.0, "/b/", 2)
		for _, line := range strings.Split(result, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, helpers.HLS_SEGMENT_FILENAME_PREFIX) ||
				(strings.HasPrefix(trimmed, "#EXT-X-MAP:") && strings.Contains(trimmed, helpers.HLS_INIT_FILENAME)) {
				if !strings.Contains(trimmed, "audio_track=2") {
					t.Errorf("Expected audio_track=2 in line: %q", trimmed)
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
}