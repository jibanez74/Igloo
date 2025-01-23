package ffmpeg

import (
	"errors"
	"igloo/cmd/internal/helpers"
)

const (
	DefaultVideoCodec  = "libx264"           // Software encoding
	NvencVideoCodec    = "h264_nvenc"        // NVIDIA GPU
	QsvVideoCodec      = "h264_qsv"          // Intel QuickSync
	VtVideoCodec       = "h264_videotoolbox" // Apple VideoToolbox
	DefaultVideoHeight = 480
	DefaultVideoRate   = 2000
	DefaultPreset      = "fast"
)

var ValidPresets = [9]string{
	"ultrafast",
	"superfast",
	"veryfast",
	"faster",
	"fast",
	"medium",
	"slow",
	"slower",
	"veryslow",
}

var ValidVideoProfiles = [4]string{
	"baseline",
	"main",
	"high",
	"high10",
}

var ValidVideoBitrates = [8]int{
	2000,
	4000,
	6000,
	8000,
	10000,
	12000,
	15000,
	20000,
}

var ValidVideoHeights = [4]int{
	480,
	720,
	1080,
	2160,
}

func validateVideoSettings(opts *HlsOpts, accelMethod string) error {
	err := validateVideoBitrate(opts)
	if err != nil {
		return err
	}

	err = validateVideoHeight(opts)
	if err != nil {
		return err
	}

	err = validateVideoPreset(opts)
	if err != nil {
		return err
	}

	err = validateVideoProfile(opts, accelMethod)
	if err != nil {
		return err
	}

	return nil
}

func validateVideoProfile(opts *HlsOpts, accelMethod string) error {
	if opts.VideoProfile != "" {
		if accelMethod != NoAccel && accelMethod != NVENC {
			return errors.New("video profiles are only supported with software encoding and NVENC")
		}

		if err := helpers.ValidateInArray(opts.VideoProfile, ValidVideoProfiles[:],
			"invalid video profile: must be one of: baseline, main, high, or high10"); err != nil {

			return err
		}
	}

	return nil
}

func validateVideoBitrate(opts *HlsOpts) error {
	if opts.VideoBitrate == 0 {
		opts.VideoBitrate = DefaultVideoRate
	} else if err := helpers.ValidateInArray(opts.VideoBitrate, ValidVideoBitrates[:],
		"invalid video bitrate: must be one of: 2000, 4000, 6000, 8000, 10000, 12000, 15000, or 20000 kbps"); err != nil {

		return err
	}

	return nil
}

func validateVideoHeight(opts *HlsOpts) error {
	if opts.VideoHeight == 0 {
		opts.VideoHeight = DefaultVideoHeight
	} else if err := helpers.ValidateInArray(opts.VideoHeight, ValidVideoHeights[:],
		"invalid video height: must be one of: 480, 720, 1080, or 2160"); err != nil {

		return err
	}

	return nil
}

func validateVideoPreset(opts *HlsOpts) error {
	if opts.Preset == "" {
		opts.Preset = DefaultPreset
	} else if err := helpers.ValidateInArray(opts.Preset, ValidPresets[:],
		"invalid video preset"); err != nil {

		return err
	}

	return nil
}
