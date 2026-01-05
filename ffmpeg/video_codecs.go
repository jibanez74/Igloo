package ffmpeg

type videoSettings struct {
	Resolution int
	Codec      string
	Preset     string
}

var (
	ValidPresetsMap = map[string]bool{
		VideoPresetUltrafast: true,
		VideoPresetSuperfast: true,
		VideoPresetVeryfast:  true,
		VideoPresetFaster:    true,
		VideoPresetFast:      true,
		VideoPresetMedium:    true,
		VideoPresetSlow:      true,
		VideoPresetSlower:    true,
		VideoPresetVeryslow:  true,
	}

	ValidResolutions = map[int]string{
		480:  "854x480",
		720:  "1280x720",
		1080: "1920x1080",
		2160: "3840x2160",
	}

	ValidCodecTypes = map[string]bool{
		VideoCodecH264: true,
		VideoCodecH265: true,
	}
)

func (f *ffmpeg) validateVideoSettings(s *videoSettings) error {
	if s == nil {
		return &ffmpegError{
			Field: "videoSettings",
			Value: s,
			Msg:   "videoSettings is nil",
		}
	}

	if !ValidPresetsMap[s.Preset] {
		return &ffmpegError{
			Field: "preset",
			Value: s.Preset,
			Msg:   "invalid preset",
		}
	}

	if !ValidCodecTypes[s.Codec] {
		return &ffmpegError{
			Field: "codec",
			Value: s.Codec,
			Msg:   "invalid codec",
		}
	}

	_, exists := ValidResolutions[s.Resolution]
	if !exists {
		return &ffmpegError{
			Field: "resolution",
			Value: s.Resolution,
			Msg:   "invalid resolution",
		}
	}

	return nil
}
