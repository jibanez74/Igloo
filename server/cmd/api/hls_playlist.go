package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"igloo/cmd/internal/helpers"
)

// buildResumePlaylist constructs a complete VOD M3U8 for a resume session.
//
// Segments 0 through startSegment-1 are placeholder entries with estimated
// durations. They are never served by the segment handler (which returns 404
// for indices below startSegment), so they exist only to give hls.js a correct
// total-duration timeline for the progress bar and for seeks that trigger a
// session rebase.
//
// Segments from startSegment onwards use the accurate per-segment durations
// parsed from the FFmpeg-generated final playlist, eliminating the timing drift
// that occurs with the flat 4-second estimate used during encoding.
func buildResumePlaylist(finalPlaylist string, totalDurationSec float64, baseURL string, audioTrack int, startSegment int64) string {
	// Parse actual EXTINF durations from the FFmpeg final playlist.
	var actualDurations []float64
	for _, line := range strings.Split(finalPlaylist, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#EXTINF:") {
			continue
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(trimmed, "#EXTINF:"), ",")
		if d, err := strconv.ParseFloat(raw, 64); err == nil {
			actualDurations = append(actualDurations, d)
		}
	}

	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	totalSegs := int(math.Ceil(totalDurationSec / segDur))
	if totalSegs < 1 {
		totalSegs = 1
	}

	// TARGETDURATION must be >= the longest segment (HLS spec).
	// Take the max of the estimated segment time and any actual duration.
	maxDur := segDur
	for _, d := range actualDurations {
		if d > maxDur {
			maxDur = d
		}
	}
	targetDuration := int(math.Ceil(maxDur))
	if targetDuration < helpers.HLS_SEGMENT_TIME_SEC {
		targetDuration = helpers.HLS_SEGMENT_TIME_SEC
	}

	// Sum the actual durations FFmpeg produced.
	var sumActual float64
	for _, d := range actualDurations {
		sumActual += d
	}

	// Distribute the remaining time evenly across placeholder segments so that
	// sum(placeholders) + sum(actualDurations) == totalDurationSec.
	var placeholderPerSeg float64
	if startSegment > 0 {
		remaining := totalDurationSec - sumActual
		if remaining < 0 {
			remaining = 0
		}
		placeholderPerSeg = remaining / float64(startSegment)
	}

	suffix := "?audio_track=" + strconv.Itoa(audioTrack)

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", targetDuration))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s%s%s\"\n", baseURL, helpers.HLS_INIT_FILENAME, suffix))

	for i := 0; i < totalSegs; i++ {
		var dur float64
		if i < int(startSegment) {
			// Placeholder segment before the session start point.
			elapsed := float64(i) * placeholderPerSeg
			dur = placeholderPerSeg
			if elapsed+dur > totalDurationSec {
				dur = totalDurationSec - elapsed
			}
		} else {
			actualIdx := i - int(startSegment)
			if actualIdx < len(actualDurations) {
				dur = actualDurations[actualIdx]
			} else {
				// Fallback: FFmpeg produced fewer segments than expected.
				elapsed := float64(i) * placeholderPerSeg
				dur = placeholderPerSeg
				if elapsed+dur > totalDurationSec {
					dur = totalDurationSec - elapsed
				}
			}
		}
		if dur <= 0 {
			dur = 0.001
		}
		b.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", dur))
		b.WriteString(fmt.Sprintf("%s%s%d%s%s\n",
			baseURL, helpers.HLS_SEGMENT_FILENAME_PREFIX, i, helpers.HLS_SEGMENT_FILENAME_SUFFIX, suffix))
	}

	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

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
			initName := helpers.HLS_INIT_FILENAME
			trimmed = strings.Replace(trimmed,
				fmt.Sprintf(`URI="%s"`, initName),
				fmt.Sprintf(`URI="%s%s%s"`, baseURL, initName, suffix), 1)
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
func generateVODPlaylist(totalDurationSec float64, baseURL string, audioTrack int, copyVideo bool) string {
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
	// Transcode: FFmpeg inserts keyframes freely, so 2x the target is a safe ceiling.
	// Copy-video: FFmpeg splits only at existing keyframe boundaries; GOP sizes for
	// H.265 web content are commonly 10-12s, so use HLS_COPY_VIDEO_TARGET_DURATION.
	targetDuration := helpers.HLS_SEGMENT_TIME_SEC * 2
	if copyVideo {
		targetDuration = helpers.HLS_COPY_VIDEO_TARGET_DURATION
	}
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", targetDuration))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s%s%s\"\n", baseURL, helpers.HLS_INIT_FILENAME, suffix))

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
		b.WriteString(fmt.Sprintf("%s%s%d%s%s\n", baseURL, helpers.HLS_SEGMENT_FILENAME_PREFIX, i, helpers.HLS_SEGMENT_FILENAME_SUFFIX, suffix))
	}

	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}
