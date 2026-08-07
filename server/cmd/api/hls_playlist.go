package main

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"igloo/cmd/internal/helpers"
)

type hlsAssetQueryParams struct {
	AudioTrack      *int
	StartSec        *int
	PlaybackSession string
	Reload          string
}

func buildHLSAssetQuerySuffix(query hlsAssetQueryParams) string {
	params := url.Values{}
	if query.AudioTrack != nil {
		params.Set("audio_track", strconv.Itoa(*query.AudioTrack))
	}
	if query.StartSec != nil {
		params.Set("start", strconv.Itoa(*query.StartSec))
	}
	if strings.TrimSpace(query.PlaybackSession) != "" {
		params.Set("playback_session", strings.TrimSpace(query.PlaybackSession))
	}
	if strings.TrimSpace(query.Reload) != "" {
		params.Set("reload", strings.TrimSpace(query.Reload))
	}
	if len(params) == 0 {
		return ""
	}
	return "?" + params.Encode()
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
func rewritePlaylistURLs(playlist, baseURL, querySuffix string) string {
	var b strings.Builder
	for _, line := range strings.Split(playlist, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#EXT-X-MAP:") {
			initName := helpers.HLS_INIT_FILENAME
			trimmed = strings.Replace(trimmed,
				fmt.Sprintf(`URI="%s"`, initName),
				fmt.Sprintf(`URI="%s%s%s"`, baseURL, initName, querySuffix), 1)
			b.WriteString(trimmed)
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			b.WriteString(baseURL + trimmed + querySuffix)
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
// Only transcode sessions use it: their -force_key_frames boundaries make the
// arithmetic exact, while copy-video sessions serve FFmpeg's own playlist.
//
// FFmpeg produces the actual segment files in the background; the segment
// handler waits for each file to appear on disk before serving.
func generateVODPlaylist(totalDurationSec float64, baseURL, querySuffix string) string {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	segCount := int(math.Ceil(totalDurationSec / segDur))
	if segCount < 1 {
		segCount = 1
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	// Accurate because -force_key_frames pins an IDR to every segment boundary.
	// hls.js ignores the tag in media playlists; native HLS players (Safari)
	// use it to start decoding from any segment.
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", helpers.HLS_SEGMENT_TIME_SEC*2))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s%s%s\"\n", baseURL, helpers.HLS_INIT_FILENAME, querySuffix))

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
		b.WriteString(fmt.Sprintf("%s%s%d%s%s\n", baseURL, helpers.HLS_SEGMENT_FILENAME_PREFIX, i, helpers.HLS_SEGMENT_FILENAME_SUFFIX, querySuffix))
	}

	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}
