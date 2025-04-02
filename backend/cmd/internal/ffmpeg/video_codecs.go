package ffmpeg

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

	ValidVideoProfilesMap = map[string]bool{
		VideoProfileBaseline: true,
		VideoProfileMain:     true,
		VideoProfileHigh:     true,
		VideoProfileHigh10:   true,
	}

	ValidVideoBitratesMap = map[int]bool{
		1000:  true, // 480p low quality
		2000:  true, // 480p
		4000:  true, // 720p
		6000:  true, // 720p high quality
		8000:  true, // 1080p
		10000: true, // 1080p high quality
		12000: true, // 1080p very high quality
		15000: true, // 4K
		20000: true, // 4K high quality
	}

	ValidVideoHeightsMap = map[int]bool{
		480:  true, // 480p
		720:  true, // 720p
		1080: true, // 1080p
		2160: true, // 4K
	}

	ValidCodecTypes = map[string]bool{
		VideoCodecH264: true,
		VideoCodecH265: true,
	}
)

func validateVideoSettings(opts *HlsOpts, enableHardwareEncoding bool) error {
	if opts == nil {
		return &ffmpegError{
			Field: "options",
			Value: nil,
			Msg:   "video options cannot be nil",
		}
	}

	if opts.VideoCodec == VideoCodecCopy {
		return nil
	}

	if !ValidCodecTypes[opts.VideoCodec] {
		return &ffmpegError{
			Field: "codec",
			Value: opts.VideoCodec,
			Msg:   "unsupported video codec",
		}
	}

	if !ValidVideoBitratesMap[opts.VideoBitrate] {
		return &ffmpegError{
			Field: "bitrate",
			Value: opts.VideoBitrate,
			Msg:   "unsupported video bitrate",
		}
	}

	if !ValidVideoHeightsMap[opts.VideoHeight] {
		return &ffmpegError{
			Field: "height",
			Value: opts.VideoHeight,
			Msg:   "unsupported video resolution",
		}
	}

	if !ValidPresetsMap[opts.Preset] {
		return &ffmpegError{
			Field: "preset",
			Value: opts.Preset,
			Msg:   "unsupported encoding preset",
		}
	}

	if opts.VideoProfile != "" {
		// Only allow video profiles when hardware encoding is enabled
		if !enableHardwareEncoding {
			return &ffmpegError{
				Field: "profile",
				Value: opts.VideoProfile,
				Msg:   "video profiles are only supported with hardware encoding",
			}
		}

		if !ValidVideoProfilesMap[opts.VideoProfile] {
			return &ffmpegError{
				Field: "profile",
				Value: opts.VideoProfile,
				Msg:   "unsupported video profile",
			}
		}

		switch opts.VideoProfile {
		case VideoProfileHigh10:
			// High 10 profile requires H.264
			if opts.VideoCodec != VideoCodecH264 {
				return &ffmpegError{
					Field: "profile",
					Value: opts.VideoProfile,
					Msg:   "high10 profile is only supported with H.264",
				}
			}
		}
	}

	if opts.VideoHeight == 2160 && opts.VideoBitrate < 15000 {
		return &ffmpegError{
			Field: "bitrate",
			Value: opts.VideoBitrate,
			Msg:   "4K resolution requires minimum bitrate of 15000kbps",
		}
	}

	return nil
}
