package helpers

// HLSProfileConfig holds encoding parameters for one HLS profile.
type HLSProfileConfig struct {
	ID         string // profile id (e.g. 1080p_4mbps)
	Width      int    // target width (0 = use height and preserve aspect)
	Height     int    // target height
	VideoBitrate string // e.g. "8M", "4M"
	Bufsize    string // e.g. "16M", "8M"
}

// HLSAllowedProfiles is the ordered list of profile IDs allowed in requests.
// HLS_PROFILE_REMUX copies the video stream and transcodes only audio;
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
	HLS_PROFILE_2160P_16MBPS: {ID: HLS_PROFILE_2160P_16MBPS, Width: 3840, Height: 2160, VideoBitrate: "16M", Bufsize: "32M"},
	HLS_PROFILE_1080P_8MBPS:  {ID: HLS_PROFILE_1080P_8MBPS, Width: 1920, Height: 1080, VideoBitrate: "8M", Bufsize: "16M"},
	HLS_PROFILE_1080P_6MBPS:  {ID: HLS_PROFILE_1080P_6MBPS, Width: 1920, Height: 1080, VideoBitrate: "6M", Bufsize: "12M"},
	HLS_PROFILE_1080P_4MBPS:  {ID: HLS_PROFILE_1080P_4MBPS, Width: 1920, Height: 1080, VideoBitrate: "4M", Bufsize: "8M"},
	HLS_PROFILE_720P_3MBPS:   {ID: HLS_PROFILE_720P_3MBPS, Width: 1280, Height: 720, VideoBitrate: "3M", Bufsize: "6M"},
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
