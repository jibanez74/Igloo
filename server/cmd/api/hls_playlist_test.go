package main

import (
	"fmt"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestFinalizeEventPlaylist(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		wantEnd  bool
	}{
		{
			name: "converts EVENT to VOD and appends ENDLIST",
			input: "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n",
			wantType: "#EXT-X-PLAYLIST-TYPE:VOD",
			wantEnd:  true,
		},
		{
			name: "preserves existing ENDLIST",
			input: "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n",
			wantType: "#EXT-X-PLAYLIST-TYPE:VOD",
			wantEnd:  true,
		},
		{
			name:     "no EVENT tag leaves playlist type unchanged",
			input:    "#EXTM3U\n#EXTINF:4.0,\nsegment_0.m4s\n",
			wantType: "",
			wantEnd:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := finalizeEventPlaylist(tt.input)

			if tt.wantType != "" && !strings.Contains(got, tt.wantType) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantType, got)
			}
			if tt.wantType != "" && strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:EVENT") {
				t.Error("EVENT type should have been replaced with VOD")
			}
			if tt.wantEnd && !strings.Contains(got, "#EXT-X-ENDLIST") {
				t.Error("expected #EXT-X-ENDLIST in output")
			}
		})
	}
}

func TestFinalizeEventPlaylist_DoesNotDuplicateEndlist(t *testing.T) {
	input := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nseg.m4s\n#EXT-X-ENDLIST\n"
	got := finalizeEventPlaylist(input)

	count := strings.Count(got, "#EXT-X-ENDLIST")
	if count != 1 {
		t.Errorf("expected exactly 1 #EXT-X-ENDLIST, found %d", count)
	}
}

func TestRewritePlaylistURLs(t *testing.T) {
	playlist := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-VERSION:7",
		"#EXT-X-TARGETDURATION:8",
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		fmt.Sprintf(`#EXT-X-MAP:URI="%s"`, helpers.HLS_INIT_FILENAME),
		"#EXTINF:4.000000,",
		"segment_0.m4s",
		"#EXTINF:4.000000,",
		"segment_1.m4s",
		"#EXT-X-ENDLIST",
		"",
	}, "\n")

	baseURL := "/api/movies/7/hls/1080p_8mbps/"
	audioTrack := 2

	got := rewritePlaylistURLs(playlist, baseURL, audioTrack)

	wantInitURI := fmt.Sprintf(`URI="%s%s?audio_track=%d"`, baseURL, helpers.HLS_INIT_FILENAME, audioTrack)
	if !strings.Contains(got, wantInitURI) {
		t.Errorf("expected init URI %q in output, got:\n%s", wantInitURI, got)
	}

	wantSeg0 := fmt.Sprintf("%ssegment_0.m4s?audio_track=%d", baseURL, audioTrack)
	if !strings.Contains(got, wantSeg0) {
		t.Errorf("expected segment 0 URL %q in output, got:\n%s", wantSeg0, got)
	}

	wantSeg1 := fmt.Sprintf("%ssegment_1.m4s?audio_track=%d", baseURL, audioTrack)
	if !strings.Contains(got, wantSeg1) {
		t.Errorf("expected segment 1 URL %q in output, got:\n%s", wantSeg1, got)
	}

	if !strings.Contains(got, "#EXTM3U") {
		t.Error("M3U8 header tag should be preserved")
	}
	if !strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Error("ENDLIST tag should be preserved")
	}
}

func TestRewritePlaylistURLs_PreservesMetadataLines(t *testing.T) {
	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:8\n#EXT-X-ENDLIST\n"
	got := rewritePlaylistURLs(playlist, "/base/", 0)

	for _, tag := range []string{"#EXTM3U", "#EXT-X-VERSION:7", "#EXT-X-TARGETDURATION:8", "#EXT-X-ENDLIST"} {
		if !strings.Contains(got, tag) {
			t.Errorf("expected metadata line %q preserved in output", tag)
		}
	}
}

func TestGenerateVODPlaylist(t *testing.T) {
	tests := []struct {
		name       string
		durationSec float64
		audioTrack int
		wantSegs   int
	}{
		{
			name:        "short movie (one segment)",
			durationSec: 3.0,
			audioTrack:  0,
			wantSegs:    1,
		},
		{
			name:        "exactly one segment boundary",
			durationSec: float64(helpers.HLS_SEGMENT_TIME_SEC),
			audioTrack:  0,
			wantSegs:    1,
		},
		{
			name:        "two full segments",
			durationSec: float64(helpers.HLS_SEGMENT_TIME_SEC * 2),
			audioTrack:  0,
			wantSegs:    2,
		},
		{
			name:        "partial last segment",
			durationSec: float64(helpers.HLS_SEGMENT_TIME_SEC)*2 + 1,
			audioTrack:  0,
			wantSegs:    3,
		},
		{
			name:        "alternate audio track in URLs",
			durationSec: float64(helpers.HLS_SEGMENT_TIME_SEC),
			audioTrack:  3,
			wantSegs:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL := "/api/movies/1/hls/720p_3mbps/"
			got := generateVODPlaylist(tt.durationSec, baseURL, tt.audioTrack)

			if !strings.HasPrefix(got, "#EXTM3U\n") {
				t.Error("playlist must start with #EXTM3U")
			}
			if !strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:VOD") {
				t.Error("playlist must declare VOD type")
			}
			if !strings.Contains(got, "#EXT-X-ENDLIST") {
				t.Error("VOD playlist must include #EXT-X-ENDLIST")
			}

			initURI := fmt.Sprintf(`URI="%s%s?audio_track=%d"`, baseURL, helpers.HLS_INIT_FILENAME, tt.audioTrack)
			if !strings.Contains(got, initURI) {
				t.Errorf("expected init map URI %q", initURI)
			}

			segCount := strings.Count(got, "#EXTINF:")
			if segCount != tt.wantSegs {
				t.Errorf("expected %d segments, got %d", tt.wantSegs, segCount)
			}

			suffix := fmt.Sprintf("?audio_track=%d", tt.audioTrack)
			for i := 0; i < tt.wantSegs; i++ {
				segLine := fmt.Sprintf("%s%s%d%s%s",
					baseURL,
					helpers.HLS_SEGMENT_FILENAME_PREFIX,
					i,
					helpers.HLS_SEGMENT_FILENAME_SUFFIX,
					suffix,
				)
				if !strings.Contains(got, segLine) {
					t.Errorf("expected segment line %q in playlist", segLine)
				}
			}
		})
	}
}

func TestBuildResumePlaylist_UsesActualDurations(t *testing.T) {
	// Simulate a resume session starting at segment 10 (40s into a 60s movie).
	startSegment := int64(10)
	// FFmpeg produced 5 segments with actual durations (keyframe-aligned).
	finalPlaylist := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-VERSION:7",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		fmt.Sprintf(`#EXT-X-MAP:URI="%s"`, helpers.HLS_INIT_FILENAME),
		"#EXTINF:6.000000,",
		"segment_0.m4s",
		"#EXTINF:5.000000,",
		"segment_1.m4s",
		"#EXTINF:4.000000,",
		"segment_2.m4s",
		"#EXTINF:4.000000,",
		"segment_3.m4s",
		"#EXTINF:2.000000,",
		"segment_4.m4s",
		"#EXT-X-ENDLIST",
		"",
	}, "\n")

	baseURL := "/api/movies/5/hls/remux/"
	audioTrack := 0
	totalDuration := float64(helpers.HLS_SEGMENT_TIME_SEC)*15 // 60s total

	got := buildResumePlaylist(finalPlaylist, totalDuration, baseURL, audioTrack, startSegment)

	if !strings.HasPrefix(got, "#EXTM3U\n") {
		t.Error("playlist must start with #EXTM3U")
	}
	if !strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:VOD") {
		t.Error("playlist must declare VOD type")
	}
	if !strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Error("playlist must include #EXT-X-ENDLIST")
	}

	// TARGETDURATION must be >= longest actual segment (6s here).
	if !strings.Contains(got, "#EXT-X-TARGETDURATION:6") {
		t.Errorf("TARGETDURATION should be 6 (longest actual segment), got:\n%s", got)
	}

	// Placeholder segments 0-9 must use logical indices in URLs.
	for i := 0; i < int(startSegment); i++ {
		want := fmt.Sprintf("%s%s%d%s?audio_track=%d", baseURL, helpers.HLS_SEGMENT_FILENAME_PREFIX, i, helpers.HLS_SEGMENT_FILENAME_SUFFIX, audioTrack)
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder segment URL %q in playlist", want)
		}
	}

	// Actual segments must use logical indices (startSegment + disk index).
	wantDurations := []string{"6.000000", "5.000000", "4.000000", "4.000000", "2.000000"}
	lines := strings.Split(got, "\n")
	var infLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXTINF:") {
			infLines = append(infLines, line)
		}
	}
	for i, wantDur := range wantDurations {
		actualLine := infLines[int(startSegment)+i]
		if !strings.Contains(actualLine, wantDur) {
			t.Errorf("segment %d: expected duration %s, got line %q", int(startSegment)+i, wantDur, actualLine)
		}
	}

	// Segment URLs for actual segments must use logical indices.
	wantSeg10 := fmt.Sprintf("%s%s%d%s?audio_track=%d", baseURL, helpers.HLS_SEGMENT_FILENAME_PREFIX, 10, helpers.HLS_SEGMENT_FILENAME_SUFFIX, audioTrack)
	if !strings.Contains(got, wantSeg10) {
		t.Errorf("expected logical segment URL %q for first actual segment, got:\n%s", wantSeg10, got)
	}
}

func TestBuildResumePlaylist_TargetDurationFallsBackToEstimate(t *testing.T) {
	// When all actual segments are <= HLS_SEGMENT_TIME_SEC, target duration
	// should still be at least HLS_SEGMENT_TIME_SEC.
	finalPlaylist := "#EXTM3U\n#EXTINF:3.000000,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
	got := buildResumePlaylist(finalPlaylist, 20, "/base/", 0, 1)
	want := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", helpers.HLS_SEGMENT_TIME_SEC)
	if !strings.Contains(got, want) {
		t.Errorf("expected %q, got:\n%s", want, got)
	}
}

func TestBuildResumePlaylist_ZeroStartSegmentIsFullPlaylist(t *testing.T) {
	// startSegment=0 means every segment uses actual durations (no placeholders).
	finalPlaylist := "#EXTM3U\n#EXTINF:5.000000,\nsegment_0.m4s\n#EXTINF:4.000000,\nsegment_1.m4s\n#EXT-X-ENDLIST\n"
	got := buildResumePlaylist(finalPlaylist, float64(helpers.HLS_SEGMENT_TIME_SEC)*2, "/base/", 0, 0)
	if !strings.Contains(got, "#EXTINF:5.000000,") {
		t.Error("expected actual duration 5.0 for first segment")
	}
	if !strings.Contains(got, "#EXTINF:4.000000,") {
		t.Error("expected actual duration 4.0 for second segment")
	}
}

func TestGenerateVODPlaylist_ZeroDuration(t *testing.T) {
	got := generateVODPlaylist(0, "/base/", 0)

	if !strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Error("zero-duration playlist must still be valid with ENDLIST")
	}
	if strings.Count(got, "#EXTINF:") < 1 {
		t.Error("zero-duration playlist must produce at least one segment")
	}
}

func TestGenerateVODPlaylist_TargetDurationIsDoubleSegmentTime(t *testing.T) {
	got := generateVODPlaylist(100, "/base/", 0)

	want := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", helpers.HLS_SEGMENT_TIME_SEC*2)
	if !strings.Contains(got, want) {
		t.Errorf("expected target duration %q (2x segment time), got:\n%s", want, got)
	}
}

func TestGenerateVODPlaylist_LastSegmentDurationCapped(t *testing.T) {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	totalDur := segDur + 1.5

	got := generateVODPlaylist(totalDur, "/base/", 0)

	lines := strings.Split(got, "\n")
	var infDurations []string
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXTINF:") {
			infDurations = append(infDurations, line)
		}
	}

	if len(infDurations) != 2 {
		t.Fatalf("expected 2 EXTINF lines, got %d", len(infDurations))
	}

	lastInf := infDurations[len(infDurations)-1]
	if !strings.HasPrefix(lastInf, "#EXTINF:1.5") {
		t.Errorf("last segment should have duration 1.5, got %q", lastInf)
	}
}
