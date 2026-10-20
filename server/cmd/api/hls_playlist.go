package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

type hlsAssetQueryParams struct {
	AudioTrack *int
	// AudioProfile propagates the normalized explicit audio pair onto every
	// rewritten init/segment URL so asset requests compute the same session
	// key as the manifest. Nil (legacy) adds nothing.
	AudioProfile    *helpers.HLSAudioProfileRequest
	StartSec        *int
	PlaybackSession string
	Reload          string
}

func buildHLSAssetQuerySuffix(query hlsAssetQueryParams) string {
	params := url.Values{}
	if query.AudioTrack != nil {
		params.Set("audio_track", strconv.Itoa(*query.AudioTrack))
	}
	if query.AudioProfile != nil {
		params.Set("audio_codec", string(query.AudioProfile.Codec))
		params.Set("audio_channels", strconv.Itoa(query.AudioProfile.MaxChannels))
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

// hlsStartAtBeginningTag pins a playlist's start to its first segment.
const hlsStartAtBeginningTag = "#EXT-X-START:TIME-OFFSET=0"

// startLivePlaylistAtBeginning tells players to start a copy-video session's
// live EVENT playlist at its first segment. With no #EXT-X-ENDLIST a player
// treats the playlist as live and, without a start position of its own, joins
// near the live edge, which is tens of seconds in because stream copy runs far
// ahead of real time. Native HLS (iPhone Safari) has no other way to be told;
// hls.js and the TV client pass an explicit position, which overrides the tag.
func startLivePlaylistAtBeginning(playlist string) string {
	if strings.Contains(playlist, "#EXT-X-START:") {
		return playlist
	}
	return strings.Replace(playlist, "#EXTM3U", "#EXTM3U\n"+hlsStartAtBeginningTag, 1)
}

// rewritePlaylistURLs prepends baseURL and appends the asset query suffix to
// every segment and init-map URI in a finalized playlist.
func rewritePlaylistURLs(playlist, baseURL, querySuffix string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(playlist, "\n") {
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

// generateVODPlaylist builds a complete VOD playlist for a transcode session,
// whose -force_key_frames boundaries make the arithmetic exact; copy-video
// sessions serve FFmpeg's own playlist. Every segment is listed upfront with
// #EXT-X-ENDLIST so players treat it as on-demand. ffmpeg.HLSSegmentCount sizes
// it (the tail past the last boundary folds into the final segment), and
// independentSegments comes from ffmpeg.HLSSegmentsAreIndependent.
func generateVODPlaylist(totalDurationSec float64, frameRate float64, baseURL, querySuffix string, independentSegments bool) string {
	segDur := float64(helpers.HLS_SEGMENT_TIME_SEC)
	segCount := ffmpeg.HLSSegmentCount(totalDurationSec, frameRate)

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	// Accurate when -force_key_frames pins an IDR to every segment boundary,
	// which needs the encoder to honor forced keyframes as IDRs. hls.js ignores
	// the tag in media playlists; native HLS players (Safari) use it to start
	// decoding from any segment.
	if independentSegments {
		b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	}
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", helpers.HLS_SEGMENT_TIME_SEC*2))
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s%s%s\"\n", baseURL, helpers.HLS_INIT_FILENAME, querySuffix))

	for i := 0; i < segCount; i++ {
		dur := segDur
		last := i == segCount-1
		if last {
			dur = totalDurationSec - float64(i)*segDur
		}
		if dur <= 0 {
			dur = 0.001
		}
		b.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", dur))
		b.WriteString(baseURL + hlsSegmentFilename(i) + querySuffix + "\n")
	}

	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}
