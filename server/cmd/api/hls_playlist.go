package main

import (
	"fmt"
	"igloo/cmd/internal/ffmpeg"
	"math"
	"strconv"
	"strings"
)

// generateVODPlaylist builds a complete HLS VOD M3U8 playlist from the known
// total duration. All segments are listed upfront with #EXT-X-ENDLIST so
// hls.js treats it as on-demand (starts from 0, shows full duration, allows seeking).
// FFmpeg produces the actual segment files in the background; the segment
// handler waits for each file to appear on disk before serving.
func (app *Application) generateVODPlaylist(totalDurationSec float64, baseURL string, audioTrack int) string {
	segDur := float64(ffmpeg.HLSSegmentTimeSec)
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
	targetDuration := ffmpeg.HLSSegmentTimeSec * 2
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
