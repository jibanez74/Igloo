package main

import (
	"fmt"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"
)

func testIntPtr(v int) *int {
	return &v
}

func TestBuildHLSAssetQuerySuffix(t *testing.T) {
	got := buildHLSAssetQuerySuffix(hlsAssetQueryParams{
		AudioTrack:      testIntPtr(2),
		StartSec:        testIntPtr(120),
		PlaybackSession: "4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4",
		Reload:          "7",
	})

	if got != "?audio_track=2&playback_session=4a5d0cb7-66f7-45ec-95d9-93fbe6e9eea4&reload=7&start=120" {
		t.Fatalf("buildHLSAssetQuerySuffix() = %q", got)
	}
}

func TestFinalizeEventPlaylist(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantVOD          bool
		wantEndlistCount int
	}{
		{
			name:             "converts EVENT to VOD and appends ENDLIST",
			input:            "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n",
			wantVOD:          true,
			wantEndlistCount: 1,
		},
		{
			name:             "does not duplicate an existing ENDLIST",
			input:            "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4.0,\nsegment_0.m4s\n#EXT-X-ENDLIST\n",
			wantVOD:          true,
			wantEndlistCount: 1,
		},
		{
			// No EVENT tag means the playlist is already on-demand: it must be
			// left alone apart from the ENDLIST terminator.
			name:             "no EVENT tag leaves the playlist type untouched",
			input:            "#EXTM3U\n#EXTINF:4.0,\nsegment_0.m4s\n",
			wantVOD:          false,
			wantEndlistCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := finalizeEventPlaylist(tt.input)

			if strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:EVENT") {
				t.Errorf("EVENT type survived finalization:\n%s", got)
			}
			hasVOD := strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:VOD")
			if hasVOD != tt.wantVOD {
				t.Errorf("contains VOD type = %v, want %v:\n%s", hasVOD, tt.wantVOD, got)
			}
			if count := strings.Count(got, "#EXT-X-ENDLIST"); count != tt.wantEndlistCount {
				t.Errorf("#EXT-X-ENDLIST count = %d, want %d", count, tt.wantEndlistCount)
			}
		})
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
	querySuffix := buildHLSAssetQuerySuffix(hlsAssetQueryParams{AudioTrack: testIntPtr(2)})

	got := rewritePlaylistURLs(playlist, baseURL, querySuffix)

	want := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-VERSION:7",
		"#EXT-X-TARGETDURATION:8",
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		`#EXT-X-MAP:URI="/api/movies/7/hls/1080p_8mbps/init.mp4?audio_track=2"`,
		"#EXTINF:4.000000,",
		"/api/movies/7/hls/1080p_8mbps/segment_0.m4s?audio_track=2",
		"#EXTINF:4.000000,",
		"/api/movies/7/hls/1080p_8mbps/segment_1.m4s?audio_track=2",
		"#EXT-X-ENDLIST",
		"",
		"",
	}, "\n")

	if got != want {
		t.Fatalf("rewritePlaylistURLs() =\n%s\nwant\n%s", got, want)
	}
}

func TestGenerateVODPlaylist(t *testing.T) {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)

	tests := []struct {
		name        string
		durationSec float64
		audioTrack  int
		wantSegs    int
		// wantLastEXTINF pins the trailing segment's advertised duration: a
		// partial tail must be reported at its real length, never a full
		// segment, or the player seeks past the end of the media.
		wantLastEXTINF string
	}{
		{
			name:           "short movie fits in one partial segment",
			durationSec:    3.0,
			audioTrack:     0,
			wantSegs:       1,
			wantLastEXTINF: "#EXTINF:3.000000,",
		},
		{
			name:           "duration on the segment boundary",
			durationSec:    segDur,
			audioTrack:     0,
			wantSegs:       1,
			wantLastEXTINF: "#EXTINF:4.000000,",
		},
		{
			name:           "two full segments",
			durationSec:    segDur * 2,
			audioTrack:     0,
			wantSegs:       2,
			wantLastEXTINF: "#EXTINF:4.000000,",
		},
		{
			name:           "partial last segment",
			durationSec:    segDur*2 + 1,
			audioTrack:     0,
			wantSegs:       3,
			wantLastEXTINF: "#EXTINF:1.000000,",
		},
		{
			name:           "fractional partial last segment",
			durationSec:    segDur + 1.5,
			audioTrack:     0,
			wantSegs:       2,
			wantLastEXTINF: "#EXTINF:1.500000,",
		},
		{
			// A zero-duration movie must still yield a playable manifest rather
			// than an empty one the player rejects outright.
			name:           "zero duration still yields one segment",
			durationSec:    0,
			audioTrack:     0,
			wantSegs:       1,
			wantLastEXTINF: "#EXTINF:0.001000,",
		},
		{
			name:           "alternate audio track in URLs",
			durationSec:    segDur,
			audioTrack:     3,
			wantSegs:       1,
			wantLastEXTINF: "#EXTINF:4.000000,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL := "/api/movies/1/hls/720p_3mbps/"
			querySuffix := buildHLSAssetQuerySuffix(hlsAssetQueryParams{AudioTrack: &tt.audioTrack})
			got := generateVODPlaylist(tt.durationSec, baseURL, querySuffix)

			if !strings.HasPrefix(got, "#EXTM3U\n") {
				t.Error("playlist must start with #EXTM3U")
			}
			if !strings.Contains(got, "#EXT-X-PLAYLIST-TYPE:VOD") {
				t.Error("playlist must declare VOD type")
			}
			if !strings.Contains(got, "#EXT-X-ENDLIST") {
				t.Error("VOD playlist must include #EXT-X-ENDLIST")
			}

			// Transcode sessions force keyframes on segment boundaries, so the
			// target duration is twice the nominal segment time.
			wantTarget := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", helpers.HLS_SEGMENT_TIME_SEC*2)
			if !strings.Contains(got, wantTarget) {
				t.Errorf("expected target duration %q, got:\n%s", wantTarget, got)
			}

			initURI := fmt.Sprintf(`URI="%s%s%s"`, baseURL, helpers.HLS_INIT_FILENAME, querySuffix)
			if !strings.Contains(got, initURI) {
				t.Errorf("expected init map URI %q", initURI)
			}

			var extinf []string
			for _, line := range strings.Split(got, "\n") {
				if strings.HasPrefix(line, "#EXTINF:") {
					extinf = append(extinf, line)
				}
			}
			if len(extinf) != tt.wantSegs {
				t.Fatalf("segment count = %d, want %d:\n%s", len(extinf), tt.wantSegs, got)
			}
			if last := extinf[len(extinf)-1]; last != tt.wantLastEXTINF {
				t.Errorf("last segment = %q, want %q", last, tt.wantLastEXTINF)
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
