package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"igloo/cmd/internal/helpers"
)

// finalizeEventPlaylist converts FFmpeg's event-mode playlist into a VOD
// playlist by switching the playlist type to VOD and appending #EXT-X-ENDLIST.
// The returned playlist uses relative filenames (init.mp4, segment_N.m4s);
// the manifest handler rewrites them with the correct base URL and query params.
func finalizeEventPlaylist(raw string) string {
	out := strings.Replace(raw, "#EXT-X-PLAYLIST-TYPE:EVENT", "#EXT-X-PLAYLIST-TYPE:VOD", 1)
	if !strings.Contains(out, "#EXT-X-ENDLIST") {
		out = strings.TrimRight(out, "\n") + "\n#EXT-X-ENDLIST\n"
	}
	return out
}

// rewritePlaylistURLs prepends baseURL and appends the audio_track query
// parameter to every segment and init-map URI in a finalized playlist.
func rewritePlaylistURLs(playlist, baseURL string, audioTrack int) string {
	suffix := "?audio_track=" + strconv.Itoa(audioTrack)
	var b strings.Builder
	for _, line := range strings.Split(playlist, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#EXT-X-MAP:") {
			trimmed = strings.Replace(trimmed,
				`URI="init.mp4"`,
				fmt.Sprintf(`URI="%sinit.mp4%s"`, baseURL, suffix), 1)
			b.WriteString(trimmed)
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			b.WriteString(baseURL + trimmed + suffix)
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// generateVODPlaylist builds a complete HLS VOD M3U8 playlist from the known
// total duration. All segments are listed upfront with #EXT-X-ENDLIST so
// hls.js treats it as on-demand (starts from 0, shows full duration, allows seeking).
//
// FFmpeg produces the actual segment files in the background; the segment
// handler waits for each file to appear on disk before serving.
func generateVODPlaylist(totalDurationSec float64, baseURL string, audioTrack int) string {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	segCount := int(math.Ceil(totalDurationSec / segDur))
	if segCount < 1 {
		segCount = 1
	}

	suffix := "?audio_track=" + strconv.Itoa(audioTrack)

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	// TARGETDURATION must be >= the longest segment (HLS spec).
	// With -c:v copy, FFmpeg splits at keyframes so segments can exceed the target.
	// Use 2x the target as a safe ceiling.
	targetDuration := helpers.HLS_SEGMENT_TIME_SEC * 2
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", targetDuration))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%sinit.mp4%s\"\n", baseURL, suffix))

	for i := 0; i < segCount; i++ {
		dur := segDur
		elapsed := float64(i) * segDur
		if elapsed+dur > totalDurationSec {
			dur = totalDurationSec - elapsed
		}
		if dur <= 0 {
			dur = 0.001
		}
		b.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", dur))
		b.WriteString(fmt.Sprintf("%ssegment_%d.m4s%s\n", baseURL, i, suffix))
	}

	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}
