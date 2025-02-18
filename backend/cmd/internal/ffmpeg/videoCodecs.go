package ffmpeg

const (
	DefaultVideoCodec  = "libx264"
	NvencVideoCodec    = "h264_nvenc"
	QsvVideoCodec      = "h264_qsv"
	VtVideoCodec       = "h264_videotoolbox"
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
)

func validateVideoSettings(opts *HlsOpts, accelMethod string) {
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

	if opts.VideoProfile != "" && (accelMethod != NoAccel && accelMethod != NVENC) {
		opts.VideoProfile = ""
	}

	if !ValidVideoProfilesMap[opts.VideoProfile] {
		opts.VideoProfile = ""
	}
}
