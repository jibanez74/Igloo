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
var HLSAllowedProfiles = []string{
	HLS_PROFILE_1080P_8MBPS,
	HLS_PROFILE_1080P_4MBPS,
	HLS_PROFILE_720P_3MBPS,
}

// HLSProfileConfigs maps profile ID to config for FFmpeg arg building.
var HLSProfileConfigs = map[string]HLSProfileConfig{
	HLS_PROFILE_1080P_8MBPS: {ID: HLS_PROFILE_1080P_8MBPS, Width: 1920, Height: 1080, VideoBitrate: "8M", Bufsize: "16M"},
	HLS_PROFILE_1080P_4MBPS: {ID: HLS_PROFILE_1080P_4MBPS, Width: 1920, Height: 1080, VideoBitrate: "4M", Bufsize: "8M"},
	HLS_PROFILE_720P_3MBPS:  {ID: HLS_PROFILE_720P_3MBPS, Width: 1280, Height: 720, VideoBitrate: "3M", Bufsize: "6M"},
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
