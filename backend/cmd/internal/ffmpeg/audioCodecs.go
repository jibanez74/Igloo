package ffmpeg

const (
	DefaultAudioRate     = 128
	DefaultAudioChannels = 2
	DefaultAudioCodec    = "aac"
	AudioCodecCopy       = "copy"
	AudioCodecAAC        = "aac"
	AudioCodecAC3        = "ac3"
	AudioCodecFLAC       = "flac"
	AudioCodecMP3        = "mp3"
)

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

	ValidAudioBitratesMap = map[int]bool{
		128: true, // Standard quality
		192: true, // High quality
		256: true, // Very high quality
		320: true, // Maximum quality for lossy codecs
		640: true, // High quality for lossless codecs
	}
)

func validateAudioSettings(opts *HlsOpts) error {
	if opts == nil {
		return &ffmpegError{
			Field: "options",
			Value: nil,
			Msg:   "audio options cannot be nil",
		}
	}

	if !ValidAudioCodecsMap[opts.AudioCodec] {
		return &ffmpegError{
			Field: "codec",
			Value: opts.AudioCodec,
			Msg:   "unsupported audio codec",
		}
	}

	if opts.AudioCodec == AudioCodecCopy {
		return nil
	}

	if !ValidAudioBitratesMap[opts.AudioBitRate] {
		return &ffmpegError{
			Field: "bitrate",
			Value: opts.AudioBitRate,
			Msg:   "unsupported audio bitrate",
		}
	}

	if !ValidAudioChannelsMap[opts.AudioChannels] {
		return &ffmpegError{
			Field: "channels",
			Value: opts.AudioChannels,
			Msg:   "unsupported audio channel configuration",
		}
	}

	switch opts.AudioCodec {
	case AudioCodecFLAC:
		if opts.AudioBitRate < 256 {
			return &ffmpegError{
				Field: "bitrate",
				Value: opts.AudioBitRate,
				Msg:   "FLAC requires minimum bitrate of 256kbps",
			}
		}

	case AudioCodecMP3:
		if opts.AudioBitRate > 320 {
			return &ffmpegError{
				Field: "bitrate",
				Value: opts.AudioBitRate,
				Msg:   "MP3 maximum bitrate is 320kbps",
			}
		}
	}

	return nil
}
