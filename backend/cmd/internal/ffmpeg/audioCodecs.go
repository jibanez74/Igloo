package ffmpeg

const (
	DefaultAudioRate     = 128
	DefaultAudioChannels = 2
	DefaultAudioCodec    = "aac"
)

var ValidAudioCodecs = [3]string{
	"copy",
	"aac",
	"mp3",
}

var ValidAudioChannels = [4]int{
	1,
	2,
	6,
	8,
}

var ValidAudioBitrates = [4]int{
	128,
	192,
	256,
	320,
}

func validateAudioSettings(opts *HlsOpts) error {
	err := validateAudioCodec(opts)
	if err != nil {
		return err
	}

	err = validateAudioBitrate(opts)
	if err != nil {
		return err
	}

	err = validateAudioChannels(opts)
	if err != nil {
		return err
	}

	return nil
}

func validateAudioCodec(opts *HlsOpts) error {
	if opts.AudioCodec == "" {
		opts.AudioCodec = DefaultAudioCodec
	} else if err := validateInArray(opts.AudioCodec, ValidAudioCodecs[:],
		"invalid audio codec: must be one of: copy, aac, or mp3"); err != nil {

		return err
	}

	return nil
}

func validateAudioBitrate(opts *HlsOpts) error {
	if opts.AudioCodec != "copy" {
		if opts.AudioBitRate == 0 {
			opts.AudioBitRate = DefaultAudioRate
		} else if err := validateInArray(opts.AudioBitRate, ValidAudioBitrates[:],
			"invalid audio bitrate: must be one of: 128, 192, 256, or 320 kbps"); err != nil {

			return err
		}
	}

	return nil
}

func validateAudioChannels(opts *HlsOpts) error {
	if opts.AudioChannels == 0 {
		opts.AudioChannels = DefaultAudioChannels
	} else if err := validateInArray(opts.AudioChannels, ValidAudioChannels[:],
		"invalid audio channels: must be one of: 1 (mono), 2 (stereo), 6 (5.1), 8 (7.1)"); err != nil {

		return err
	}

	return nil
}
