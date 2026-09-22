package helpers

import "slices"

// bitsPerMegabit converts a profile's VideoMbps to the bits-per-second unit
// ffprobe reports a stream bitrate in.
const bitsPerMegabit = 1_000_000

// HLSProfileConfig holds encoding parameters for one HLS profile. Scaling
// uses only Height (`scale=-2:<height>` preserves the source aspect ratio).
type HLSProfileConfig struct {
	ID     string // profile id (e.g. 1080p_4mbps)
	Height int    // target height
	// VideoBitrate is the value handed to FFmpeg verbatim; VideoMbps is the
	// same number for code that has to compare it. A test keeps them in step.
	VideoBitrate string // e.g. "8M", "4M"
	VideoMbps    int    // e.g. 8, 4
	Bufsize      string // e.g. "16M", "8M"
}

// HLS profile identifiers are URL-visible values accepted by requests. The
// transcode ids are the keys of HLSProfileConfigs below; HLS_PROFILE_REMUX has
// no entry there and is also read by cmd/api and ffmpeg.
const (
	HLS_PROFILE_REMUX        = "remux"
	HLS_PROFILE_2160P_16MBPS = "2160p_16mbps"
	HLS_PROFILE_1080P_8MBPS  = "1080p_8mbps"
	HLS_PROFILE_1080P_6MBPS  = "1080p_6mbps"
	HLS_PROFILE_1080P_4MBPS  = "1080p_4mbps"
	HLS_PROFILE_720P_3MBPS   = "720p_3mbps"
)

// HLSAllowedProfiles is the ordered list of profile IDs allowed in requests.
// HLS_PROFILE_REMUX copies the video stream and re-maps the selected audio track,
// copying it when it is already AAC and transcoding to stereo AAC otherwise;
// it has no entry in HLSProfileConfigs because there are no resolution/bitrate constraints.
var HLSAllowedProfiles = []string{
	HLS_PROFILE_REMUX,
	HLS_PROFILE_2160P_16MBPS,
	HLS_PROFILE_1080P_8MBPS,
	HLS_PROFILE_1080P_6MBPS,
	HLS_PROFILE_1080P_4MBPS,
	HLS_PROFILE_720P_3MBPS,
}

// HLSProfileConfigs maps profile ID to config for FFmpeg arg building.
var HLSProfileConfigs = map[string]HLSProfileConfig{
	HLS_PROFILE_2160P_16MBPS: {ID: HLS_PROFILE_2160P_16MBPS, Height: 2160, VideoBitrate: "16M", VideoMbps: 16, Bufsize: "32M"},
	HLS_PROFILE_1080P_8MBPS:  {ID: HLS_PROFILE_1080P_8MBPS, Height: 1080, VideoBitrate: "8M", VideoMbps: 8, Bufsize: "16M"},
	HLS_PROFILE_1080P_6MBPS:  {ID: HLS_PROFILE_1080P_6MBPS, Height: 1080, VideoBitrate: "6M", VideoMbps: 6, Bufsize: "12M"},
	HLS_PROFILE_1080P_4MBPS:  {ID: HLS_PROFILE_1080P_4MBPS, Height: 1080, VideoBitrate: "4M", VideoMbps: 4, Bufsize: "8M"},
	HLS_PROFILE_720P_3MBPS:   {ID: HLS_PROFILE_720P_3MBPS, Height: 720, VideoBitrate: "3M", VideoMbps: 3, Bufsize: "6M"},
}

// IsAllowedHLSProfile returns true if profile is in the allowed list.
func IsAllowedHLSProfile(profile string) bool {
	return slices.Contains(HLSAllowedProfiles, profile)
}

// BestFitHLSFallbackProfile picks the transcode profile a session falls back
// to when remux is refused. Selection runs in two stages.
//
// The height stage is unchanged: the tallest configured profile that fits
// within the source height wins, and a source shorter than every profile gets
// 720p_3mbps so remux-unsafe sources still have a reliable playback option.
//
// The bitrate stage then chooses within that height. Three profiles target
// 1080 and the ordered list alone could only ever reach the first of them, so
// the highest profile bitrate that does not exceed the source bitrate wins.
// Spending more bits than the source carries buys nothing, and a 1080p source
// that failed remux prevalidation on a constrained server is exactly where the
// cheaper profiles help.
//
// Bitrate never changes the height: a low-bitrate 4K source still transcodes
// at 2160p. A source bitrate of 0 means unknown — ffprobe often omits it — and
// keeps the tallest profile at that height.
func BestFitHLSFallbackProfile(sourceHeight, sourceBitRate int64) string {
	tier := hlsFallbackTier(sourceHeight)
	if len(tier) == 0 {
		return HLS_PROFILE_720P_3MBPS
	}

	if sourceBitRate <= 0 {
		return tier[0].ID
	}

	for _, cfg := range tier {
		if int64(cfg.VideoMbps)*bitsPerMegabit <= sourceBitRate {
			return cfg.ID
		}
	}

	// Every profile at this height targets more than the source carries, so
	// take the cheapest rather than the richest: the extra bits would only
	// re-encode detail the source never had.
	return tier[len(tier)-1].ID
}

// hlsFallbackTier returns the configured profiles sharing the tallest height
// that fits within sourceHeight, in allowed-list order, which runs from the
// highest bitrate to the lowest. It is empty when the source is shorter than
// every configured profile.
func hlsFallbackTier(sourceHeight int64) []HLSProfileConfig {
	var tier []HLSProfileConfig

	for _, profileID := range HLSAllowedProfiles {
		if profileID == HLS_PROFILE_REMUX {
			continue
		}

		// A profile id with no configured height would otherwise match every
		// source, because the zero value satisfies `sourceHeight >= 0`.
		cfg, ok := HLSProfileConfigs[profileID]
		if !ok || cfg.Height <= 0 {
			continue
		}

		if len(tier) == 0 {
			if sourceHeight >= int64(cfg.Height) {
				tier = append(tier, cfg)
			}

			continue
		}

		if cfg.Height != tier[0].Height {
			break
		}

		tier = append(tier, cfg)
	}

	return tier
}

func IsBrowserCompatibleH264(codec string) bool {
	switch normalizeCodec(codec) {
	case "h264", "h.264", "avc", "avc1":
		return true
	default:
		return false
	}
}
