package helpers

// HLSProfileConfig holds encoding parameters for one HLS profile. Scaling
// uses only Height (`scale=-2:<height>` preserves the source aspect ratio).
type HLSProfileConfig struct {
	ID           string // profile id (e.g. 1080p_4mbps)
	Height       int    // target height
	VideoBitrate string // e.g. "8M", "4M"
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
	HLS_PROFILE_2160P_16MBPS: {ID: HLS_PROFILE_2160P_16MBPS, Height: 2160, VideoBitrate: "16M", Bufsize: "32M"},
	HLS_PROFILE_1080P_8MBPS:  {ID: HLS_PROFILE_1080P_8MBPS, Height: 1080, VideoBitrate: "8M", Bufsize: "16M"},
	HLS_PROFILE_1080P_6MBPS:  {ID: HLS_PROFILE_1080P_6MBPS, Height: 1080, VideoBitrate: "6M", Bufsize: "12M"},
	HLS_PROFILE_1080P_4MBPS:  {ID: HLS_PROFILE_1080P_4MBPS, Height: 1080, VideoBitrate: "4M", Bufsize: "8M"},
	HLS_PROFILE_720P_3MBPS:   {ID: HLS_PROFILE_720P_3MBPS, Height: 720, VideoBitrate: "3M", Bufsize: "6M"},
}

// IsAllowedHLSProfile returns true if profile is in the allowed list.
func IsAllowedHLSProfile(profile string) bool {
	for _, p := range HLSAllowedProfiles {
		if p == profile {
			return true
		}
	}
	return false
}

// BestFitHLSFallbackProfile returns the highest transcode profile whose max
// height fits within the source height. If the source height is smaller than
// every configured transcode profile, it falls back to the lowest-bitrate
// profile so remux-unsafe sources still have a reliable playback option.
func BestFitHLSFallbackProfile(sourceHeight int64) string {
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

		if sourceHeight >= int64(cfg.Height) {
			return profileID
		}
	}

	return HLS_PROFILE_720P_3MBPS
}

func IsBrowserCompatibleH264(codec string) bool {
	switch normalizeCodec(codec) {
	case "h264", "h.264", "avc", "avc1":
		return true
	default:
		return false
	}
}
