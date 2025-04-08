package ffmpeg

var (
	ValidAudioCodecsMap = map[string]bool{
		AudioCodecCopy: true,
		AudioCodecAAC:  true,
		AudioCodecAC3:  true,
		AudioCodecFLAC: true,
		AudioCodecMP3:  true,
	}

	ValidAudioChannelsMap = map[int]bool{
		1: true, // Mono
		2: true, // Stereo
		6: true, // 5.1 surround
		8: true, // 7.1 surround
	}
)

type audioSettings struct {
	Codec    string
	Channels int
}

func (f *ffmpeg) validateAudioSettings(opts *audioSettings) error {
	if opts == nil {
		return &ffmpegError{
			Field: "options",
			Value: nil,
			Msg:   "audio options cannot be nil",
		}
	}

	if !ValidAudioCodecsMap[opts.Codec] {
		return &ffmpegError{
			Field: "codec",
			Value: opts.Codec,
			Msg:   "unsupported audio codec",
		}
	}

	if !ValidAudioChannelsMap[opts.Channels] {
		return &ffmpegError{
			Field: "channels",
			Value: opts.Channels,
			Msg:   "unsupported audio channel configuration",
		}
	}

	return nil
}
