package main

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestBuildHLSAssetQuerySuffix(t *testing.T) {
	got := buildHLSAssetQuerySuffix(2, url.Values{
		"start":  {"120"},
		"reload": {"7"},
		"other":  {"ignored"},
	})

	if got != "?audio_track=2&reload=7&start=120" {
		t.Fatalf("buildHLSAssetQuerySuffix() = %q", got)
	}
}

func TestFinalizeEventPlaylist(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		wantEnd  bool
	}{
		{
			name:     "converts EVENT to VOD and appends ENDLIST",
			input:    "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n",
			wantType: "#EXT-X-PLAYLIST-TYPE:VOD",
			wantEnd:  true,
		},
		{
			name:     "preserves existing ENDLIST",
			input:    "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n",
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
	querySuffix := buildHLSAssetQuerySuffix(audioTrack, nil)

	got := rewritePlaylistURLs(playlist, baseURL, querySuffix)

	wantInitURI := fmt.Sprintf(`URI="%s%s%s"`, baseURL, helpers.HLS_INIT_FILENAME, querySuffix)
	if !strings.Contains(got, wantInitURI) {
		t.Errorf("expected init URI %q in output, got:\n%s", wantInitURI, got)
	}

	wantSeg0 := fmt.Sprintf("%ssegment_0.m4s%s", baseURL, querySuffix)
	if !strings.Contains(got, wantSeg0) {
		t.Errorf("expected segment 0 URL %q in output, got:\n%s", wantSeg0, got)
	}

	wantSeg1 := fmt.Sprintf("%ssegment_1.m4s%s", baseURL, querySuffix)
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
	got := rewritePlaylistURLs(playlist, "/base/", buildHLSAssetQuerySuffix(0, nil))

	for _, tag := range []string{"#EXTM3U", "#EXT-X-VERSION:7", "#EXT-X-TARGETDURATION:8", "#EXT-X-ENDLIST"} {
		if !strings.Contains(got, tag) {
			t.Errorf("expected metadata line %q preserved in output", tag)
		}
	}
}

func TestGenerateVODPlaylist(t *testing.T) {
	tests := []struct {
		name        string
		durationSec float64
		audioTrack  int
		wantSegs    int
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
			querySuffix := buildHLSAssetQuerySuffix(tt.audioTrack, nil)
			got := generateVODPlaylist(tt.durationSec, baseURL, querySuffix, false)

			if !strings.HasPrefix(got, "#EXTM3U\n") {
				t.Error("playlist must start with #EXTM3U")
			}
			if !strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:VOD") {
				t.Error("playlist must declare VOD type")
			}
			if !strings.Contains(got, "#EXT-X-ENDLIST") {
				t.Error("VOD playlist must include #EXT-X-ENDLIST")
			}

			initURI := fmt.Sprintf(`URI="%s%s%s"`, baseURL, helpers.HLS_INIT_FILENAME, querySuffix)
			if !strings.Contains(got, initURI) {
				t.Errorf("expected init map URI %q", initURI)
			}

			segCount := strings.Count(got, "#EXTINF:")
			if segCount != tt.wantSegs {
				t.Errorf("expected %d segments, got %d", tt.wantSegs, segCount)
			}

			for i := 0; i < tt.wantSegs; i++ {
				segLine := fmt.Sprintf("%s%s%d%s%s",
					baseURL,
					helpers.HLS_SEGMENT_FILENAME_PREFIX,
					i,
					helpers.HLS_SEGMENT_FILENAME_SUFFIX,
					querySuffix,
				)
				if !strings.Contains(got, segLine) {
					t.Errorf("expected segment line %q in playlist", segLine)
				}
			}
		})
	}
}

func TestBuildResumePlaylist_UsesActualDurations(t *testing.T) {
	startSegment := int64(10)
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
	querySuffix := buildHLSAssetQuerySuffix(audioTrack, url.Values{
		"start":  {"40"},
		"reload": {"2"},
	})
	totalDuration := float64(helpers.HLS_SEGMENT_TIME_SEC) * 15

	got := buildResumePlaylist(finalPlaylist, totalDuration, baseURL, querySuffix, startSegment)

	if !strings.HasPrefix(got, "#EXTM3U\n") {
		t.Error("playlist must start with #EXTM3U")
	}
	if !strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:VOD") {
		t.Error("playlist must declare VOD type")
	}
	if !strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Error("playlist must include #EXT-X-ENDLIST")
	}

	if !strings.Contains(got, "#EXT-X-TARGETDURATION:6") {
		t.Errorf("TARGETDURATION should be 6 (longest actual segment), got:\n%s", got)
	}

	for i := 0; i < int(startSegment); i++ {
		want := fmt.Sprintf("%s%s%d%s%s", baseURL, helpers.HLS_SEGMENT_FILENAME_PREFIX, i, helpers.HLS_SEGMENT_FILENAME_SUFFIX, querySuffix)
		if !strings.Contains(got, want) {
			t.Errorf("expected placeholder segment URL %q in playlist", want)
		}
	}

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

	wantSeg10 := fmt.Sprintf("%s%s%d%s%s", baseURL, helpers.HLS_SEGMENT_FILENAME_PREFIX, 10, helpers.HLS_SEGMENT_FILENAME_SUFFIX, querySuffix)
	if !strings.Contains(got, wantSeg10) {
		t.Errorf("expected logical segment URL %q for first actual segment, got:\n%s", wantSeg10, got)
	}

	var totalINF float64
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			raw := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "#EXTINF:"), ",")
			var d float64
			if _, err := fmt.Sscanf(raw, "%f", &d); err == nil {
				totalINF += d
			}
		}
	}
	const epsilon = 0.01
	if totalINF < totalDuration-epsilon || totalINF > totalDuration+epsilon {
		t.Errorf("sum of EXTINF durations = %.6f, want %.6f (±%.3f)", totalINF, totalDuration, epsilon)
	}
}

func TestBuildResumePlaylist_PlaceholderDurationIsConsistent(t *testing.T) {
	finalPlaylist := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		"#EXTINF:6.000000,", "segment_0.m4s",
		"#EXTINF:5.000000,", "segment_1.m4s",
		"#EXTINF:4.000000,", "segment_2.m4s",
		"#EXTINF:4.000000,", "segment_3.m4s",
		"#EXTINF:2.000000,", "segment_4.m4s",
		"#EXT-X-ENDLIST", "",
	}, "\n")

	totalDuration := 60.0
	startSegment := int64(10)
	got := buildResumePlaylist(finalPlaylist, totalDuration, "/base/", buildHLSAssetQuerySuffix(0, nil), startSegment)

	var total float64
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			raw := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "#EXTINF:"), ",")
			var d float64
			if _, err := fmt.Sscanf(raw, "%f", &d); err == nil {
				total += d
			}
		}
	}
	const epsilon = 0.01
	if total < totalDuration-epsilon || total > totalDuration+epsilon {
		t.Errorf("sum of EXTINF durations = %.6f, want %.6f (±%.3f)", total, totalDuration, epsilon)
	}

	wantPlaceholder := (totalDuration - 21.0) / float64(startSegment)
	lines := strings.Split(got, "\n")
	var infLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXTINF:") {
			infLines = append(infLines, line)
		}
	}
	for i := 0; i < int(startSegment); i++ {
		raw := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(infLines[i]), "#EXTINF:"), ",")
		var d float64
		if _, err := fmt.Sscanf(raw, "%f", &d); err != nil {
			t.Fatalf("could not parse EXTINF at index %d: %q", i, infLines[i])
		}
		if d < wantPlaceholder-epsilon || d > wantPlaceholder+epsilon {
			t.Errorf("placeholder segment %d duration = %.6f, want %.6f (±%.3f)", i, d, wantPlaceholder, epsilon)
		}
	}
}

func TestBuildResumePlaylist_ZeroStartSegmentNoPlaceholders(t *testing.T) {
	finalPlaylist := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		"#EXTINF:5.500000,", "segment_0.m4s",
		"#EXTINF:4.500000,", "segment_1.m4s",
		"#EXT-X-ENDLIST", "",
	}, "\n")

	totalDuration := 10.0
	got := buildResumePlaylist(finalPlaylist, totalDuration, "/base/", buildHLSAssetQuerySuffix(0, nil), 0)

	var total float64
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "#EXTINF:") {
			raw := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "#EXTINF:"), ",")
			var d float64
			if _, err := fmt.Sscanf(raw, "%f", &d); err == nil {
				total += d
			}
		}
	}
	const epsilon = 0.01
	if total < 10.0-epsilon || total > 10.0+epsilon {
		t.Errorf("sum of EXTINF durations = %.6f, want 10.0 (±%.3f)", total, epsilon)
	}
	if !strings.Contains(got, "#EXTINF:5.500000,") || !strings.Contains(got, "#EXTINF:4.500000,") {
		t.Error("expected actual durations 5.5 and 4.5 in playlist")
	}
}

func TestBuildResumePlaylist_TargetDurationFallsBackToEstimate(t *testing.T) {
	finalPlaylist := "#EXTM3U\n#EXTINF:3.000000,\nsegment_0.m4s\n#EXT-X-ENDLIST\n"
	got := buildResumePlaylist(finalPlaylist, 20, "/base/", buildHLSAssetQuerySuffix(0, nil), 1)
	want := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", helpers.HLS_SEGMENT_TIME_SEC)
	if !strings.Contains(got, want) {
		t.Errorf("expected %q, got:\n%s", want, got)
	}
}

func TestBuildResumePlaylist_ZeroStartSegmentIsFullPlaylist(t *testing.T) {
	finalPlaylist := "#EXTM3U\n#EXTINF:5.000000,\nsegment_0.m4s\n#EXTINF:4.000000,\nsegment_1.m4s\n#EXT-X-ENDLIST\n"
	got := buildResumePlaylist(finalPlaylist, float64(helpers.HLS_SEGMENT_TIME_SEC)*2, "/base/", buildHLSAssetQuerySuffix(0, nil), 0)
	if !strings.Contains(got, "#EXTINF:5.000000,") {
		t.Error("expected actual duration 5.0 for first segment")
	}
	if !strings.Contains(got, "#EXTINF:4.000000,") {
		t.Error("expected actual duration 4.0 for second segment")
	}
}

func TestGenerateVODPlaylist_ZeroDuration(t *testing.T) {
	got := generateVODPlaylist(0, "/base/", buildHLSAssetQuerySuffix(0, nil), false)

	if !strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Error("zero-duration playlist must still be valid with ENDLIST")
	}
	if strings.Count(got, "#EXTINF:") < 1 {
		t.Error("zero-duration playlist must produce at least one segment")
	}
}

func TestGenerateVODPlaylist_TranscodeTargetDurationIsDoubleSegmentTime(t *testing.T) {
	got := generateVODPlaylist(100, "/base/", buildHLSAssetQuerySuffix(0, nil), false)

	want := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", helpers.HLS_SEGMENT_TIME_SEC*2)
	if !strings.Contains(got, want) {
		t.Errorf("expected target duration %q (2x segment time) for transcode mode, got:\n%s", want, got)
	}
}

func TestGenerateVODPlaylist_CopyVideoUsesLargerTargetDuration(t *testing.T) {
	got := generateVODPlaylist(100, "/base/", buildHLSAssetQuerySuffix(0, nil), true)

	want := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", helpers.HLS_COPY_VIDEO_TARGET_DURATION)
	if !strings.Contains(got, want) {
		t.Errorf("expected target duration %q for copy-video mode, got:\n%s", want, got)
	}
}

func TestGenerateVODPlaylist_LastSegmentDurationCapped(t *testing.T) {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	totalDur := segDur + 1.5

	got := generateVODPlaylist(totalDur, "/base/", buildHLSAssetQuerySuffix(0, nil), false)

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
