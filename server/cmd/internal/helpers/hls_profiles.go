package helpers

// HLSProfileConfig holds resolution and bitrate for an HLS encoding profile.
type HLSProfileConfig struct {
	Width        int    // e.g. 1920
	Height       int    // e.g. 1080
	VideoBitrate string // e.g. "8000k" for FFmpeg
	ScaleFilter  string // e.g. "scale=1920:1080"
}

// HLSAllowedProfiles is the list of profile IDs allowed for HLS. 4K is deferred.
var HLSAllowedProfiles = []string{
	HLSProfile1080p8Mbps,
	HLSProfile1080p4Mbps,
	HLSProfile720p3Mbps,
}

// HLSProfileConfigs maps profile ID to resolution and bitrate for FFmpeg.
var HLSProfileConfigs = map[string]HLSProfileConfig{
	HLSProfile1080p8Mbps: {Width: 1920, Height: 1080, VideoBitrate: "8000k", ScaleFilter: "scale=1920:1080"},
	HLSProfile1080p4Mbps: {Width: 1920, Height: 1080, VideoBitrate: "4000k", ScaleFilter: "scale=1920:1080"},
	HLSProfile720p3Mbps:  {Width: 1280, Height: 720, VideoBitrate: "3000k", ScaleFilter: "scale=1280:720"},
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
