package ffmpeg

const (
	DefaultAudioRate     = 128
	DefaultAudioChannels = 2
	DefaultAudioCodec    = "aac"
)

var (
	ValidAudioCodecsMap = map[string]bool{
		"copy": true,
		"aac":  true,
		"ac3":  true,
		"flac": true,
		"mp3":  true,
	}

	ValidAudioChannelsMap = map[int]bool{
		1: true,
		2: true,
		6: true,
		8: true,
	}

	ValidAudioBitratesMap = map[int]bool{
		128: true,
		192: true,
		256: true,
		320: true,
		640: true,
	}
)

func validateAudioSettings(opts *HlsOpts) {
	if !ValidAudioCodecsMap[opts.AudioCodec] {
		opts.AudioCodec = DefaultAudioCodec
	}

	if opts.AudioCodec == "copy" {
		return
	}

	if !ValidAudioBitratesMap[opts.AudioBitRate] {
		opts.AudioBitRate = DefaultAudioRate
	}

	if !ValidAudioChannelsMap[opts.AudioChannels] {
		opts.AudioChannels = DefaultAudioChannels
	}
}
