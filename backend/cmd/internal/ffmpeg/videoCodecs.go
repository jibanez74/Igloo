package ffmpeg

const (
	DefaultVideoCodec  = "libx264"
	DefaultVideoHeight = 480
	DefaultVideoRate   = 2000
	DefaultPreset      = "fast"
)

var (
	ValidPresetsMap = map[string]bool{
		"ultrafast": true,
		"superfast": true,
		"veryfast":  true,
		"faster":    true,
		"fast":      true,
		"medium":    true,
		"slow":      true,
		"slower":    true,
		"veryslow":  true,
	}

	ValidVideoProfilesMap = map[string]bool{
		"baseline": true,
		"main":     true,
		"high":     true,
		"high10":   true,
	}

	ValidVideoBitratesMap = map[int]bool{
		2000:  true,
		4000:  true,
		6000:  true,
		8000:  true,
		10000: true,
		12000: true,
		15000: true,
		20000: true,
	}

	ValidVideoHeightsMap = map[int]bool{
		480:  true,
		720:  true,
		1080: true,
		2160: true,
	}

	// Valid codec types supported by our encoders
	ValidCodecTypes = map[string]bool{
		"h264": true,
		"h265": true,
	}
)

func validateVideoSettings(opts *HlsOpts, enableHardwareEncoding bool) {
	if opts.VideoCodec == "copy" {
		return
	}

	if !ValidVideoBitratesMap[opts.VideoBitrate] {
		opts.VideoBitrate = DefaultVideoRate
	}

	if !ValidVideoHeightsMap[opts.VideoHeight] {
		opts.VideoHeight = DefaultVideoHeight
	}

	if !ValidPresetsMap[opts.Preset] {
		opts.Preset = DefaultPreset
	}

	// Only allow video profiles when hardware encoding is enabled
	if opts.VideoProfile != "" && !enableHardwareEncoding {
		opts.VideoProfile = ""
	}

	if !ValidVideoProfilesMap[opts.VideoProfile] {
		opts.VideoProfile = ""
	}
}
