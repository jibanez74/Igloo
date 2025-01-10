package ffmpeg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type HlsOpts struct {
	InputPath        string
	OutputDir        string
	AudioStreamIndex int
	AudioCodec       string
	AudioBitRate     int
	AudioChannels    int
	VideoStreamIndex int
	VideoCodec       string
	VideoBitrate     int
	VideoHeight      int
	VideoProfile     string
	Preset           string
	HlsTime          int
	FileName         string
	HlsListSize      int
	HlsFlags         string
}

const (
	DefaultHlsTime     = 10
	DefaultVideoHeight = 720
	DefaultAudioRate   = 128
	DefaultVideoRate   = 4000
)

var ValidAudioCodecs = [7]string{
	"copy",
	"aac",
	"ac3",
	"eac3",
	"mp3",
	"opus",
	"flac",
}

var ValidAudioChannels = [5]int{
	1,
	2,
	4,
	6,
	8,
}

var ValidAudioBitrates = [5]int{
	128,
	192,
	256,
	320,
	640,
}

var ValidPresets = [9]string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}

var ValidVideoProfiles = [4]string{
	"baseline",
	"main",
	"high",
	"high10",
}

var ValidVideoCodecs = [5]string{
	"copy",
	"libx264",
	"h264_qsv",
	"h264_nvenc",
	"h264_videotoolbox",
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

var ValidVideoHeights = [5]int{
	360,
	480,
	720,
	1080,
	2160,
}

func (f *ffmpeg) CreateHlsStream(opts *HlsOpts) error {
	err := f.validateHlsOpts(opts)
	if err != nil {
		return err
	}

	cmd := f.prepareHlsCmd(opts)

	return cmd.Run()
}

func (f *ffmpeg) validateHlsOpts(opts *HlsOpts) error {
	if _, err := os.Stat(opts.InputPath); err != nil {
		return fmt.Errorf("input path error: %w", err)
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("output directory error: %w", err)
	}

	if opts.AudioStreamIndex < 0 {
		return errors.New("invalid audio stream index: must be 0 or greater")
	}
	if opts.VideoStreamIndex < 0 {
		return errors.New("invalid video stream index: must be 0 or greater")
	}

	if err := f.validateAudioSettings(opts); err != nil {
		return err
	}

	if err := f.validateVideoSettings(opts); err != nil {
		return err
	}

	return f.validateHLSSettings(opts)
}

func (f *ffmpeg) validateAudioSettings(opts *HlsOpts) error {
	if opts.AudioCodec == "" {
		opts.AudioCodec = "aac"
	} else if err := validateInArray(opts.AudioCodec, ValidAudioCodecs[:],
		"invalid audio codec: must be one of: copy, aac, ac3, eac3, mp3, opus, or flac"); err != nil {
		return err
	}

	if opts.AudioCodec != "copy" {
		switch opts.AudioCodec {
		case "flac":
			opts.AudioBitRate = 0
		case "opus":
			if err := validateOpusBitrate(opts); err != nil {
				return err
			}
		default:
			if err := validateStandardAudioBitrate(opts); err != nil {
				return err
			}
		}
	}

	if opts.AudioChannels == 0 {
		opts.AudioChannels = 2
	} else if err := validateInArray(opts.AudioChannels, ValidAudioChannels[:],
		"invalid audio channels: must be one of: 1 (mono), 2 (stereo), 4 (quad), 6 (5.1), 8 (7.1)"); err != nil {
		return err
	}

	return nil
}

func (f *ffmpeg) validateVideoSettings(opts *HlsOpts) error {
	if opts.VideoCodec == "" {
		opts.VideoCodec = "libx264"
	} else if err := validateInArray(opts.VideoCodec, ValidVideoCodecs[:],
		"invalid video codec: must be one of: copy, libx264, h264_qsv, h264_nvenc, or h264_videotoolbox"); err != nil {
		return err
	}

	if opts.VideoCodec != "copy" {
		if err := f.validateAccelCodecMatch(opts.VideoCodec); err != nil {
			return err
		}
	}

	if opts.VideoBitrate == 0 {
		opts.VideoBitrate = DefaultVideoRate
	} else if err := validateInArray(opts.VideoBitrate, ValidVideoBitrates[:],
		"invalid video bitrate: must be one of: 2000, 4000, 6000, 8000, 10000, 12000, 15000, or 20000 kbps"); err != nil {
		return err
	}

	if opts.VideoHeight == 0 {
		opts.VideoHeight = DefaultVideoHeight
	} else if err := validateInArray(opts.VideoHeight, ValidVideoHeights[:],
		"invalid video height: must be one of: 360, 480, 720, 1080, or 2160"); err != nil {
		return err
	}

	if opts.Preset == "" {
		opts.Preset = "fast"
	} else if err := validateInArray(opts.Preset, ValidPresets[:],
		"invalid video preset"); err != nil {
		return err
	}

	if opts.VideoProfile != "" {
		if err := validateInArray(opts.VideoProfile, ValidVideoProfiles[:],
			"invalid video profile: must be one of: baseline, main, high, or high10"); err != nil {
			return err
		}
	}

	return nil
}

func (f *ffmpeg) validateHLSSettings(opts *HlsOpts) error {
	if opts.HlsTime == 0 {
		opts.HlsTime = DefaultHlsTime
	}

	if opts.FileName == "" {
		return errors.New("fileName is required")
	}

	return nil
}

func (f *ffmpeg) prepareHlsCmd(opts *HlsOpts) *exec.Cmd {
	cmdArgs := []string{
		"-i", opts.InputPath,
		"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex),
		"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex),
	}

	if opts.AudioCodec == "copy" && opts.VideoCodec == "copy" {
		cmdArgs = append(cmdArgs, "-c:v", "copy", "-c:a", "copy")
	} else {
		if opts.AudioCodec == "copy" {
			cmdArgs = append(cmdArgs, "-c:a", "copy")
		} else if opts.AudioCodec != "" {
			cmdArgs = append(cmdArgs, "-c:a", opts.AudioCodec)
			if opts.AudioBitRate > 0 {
				cmdArgs = append(cmdArgs, "-b:a", fmt.Sprintf("%dk", opts.AudioBitRate))
			}
			if opts.AudioChannels > 0 {
				cmdArgs = append(cmdArgs, "-ac", fmt.Sprintf("%d", opts.AudioChannels))
			}
		}

		if opts.VideoCodec == "copy" {
			cmdArgs = append(cmdArgs, "-c:v", "copy")
		} else {
			videoCodec := "libx264"
			if f.AccelMethod != "" {
				switch f.AccelMethod {
				case NVENC:
					videoCodec = "h264_nvenc"
				case QSV:
					videoCodec = "h264_qsv"
				case VideoToolbox:
					videoCodec = "h264_videotoolbox"
				}
			}
			cmdArgs = append(cmdArgs, "-c:v", videoCodec)

			if opts.VideoBitrate > 0 {
				cmdArgs = append(cmdArgs, "-b:v", fmt.Sprintf("%dk", opts.VideoBitrate))
			}
			if opts.VideoHeight > 0 {
				cmdArgs = append(cmdArgs, "-vf", fmt.Sprintf("scale=-2:%d", opts.VideoHeight))
			}

			if opts.VideoProfile != "" && (f.AccelMethod == NoAccel || f.AccelMethod == NVENC) {
				cmdArgs = append(cmdArgs, "-profile:v", opts.VideoProfile)
			}

			if opts.Preset != "" {
				cmdArgs = append(cmdArgs, "-preset", opts.Preset)
			}
		}
	}

	cmdArgs = append(cmdArgs,
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", opts.HlsTime),
		"-hls_list_size", fmt.Sprintf("%d", opts.HlsListSize),
		"-hls_segment_type", "mpegts",
		"-hls_flags", "append_list",
	)

	cmdArgs = append(cmdArgs,
		filepath.Join(opts.OutputDir, fmt.Sprintf("%s.m3u8", opts.FileName)),
	)

	return exec.Command(f.Bin, cmdArgs...)
}

func validateInArray[T comparable](value T, validValues []T, errMsg string) error {
	for _, v := range validValues {
		if value == v {
			return nil
		}
	}

	return errors.New(errMsg)
}

func validateOpusBitrate(opts *HlsOpts) error {
	if opts.AudioBitRate == 0 {
		opts.AudioBitRate = 96
	} else if opts.AudioBitRate > 256 {
		return errors.New("opus codec works best with bitrates up to 256 kbps")
	}

	return nil
}

func validateStandardAudioBitrate(opts *HlsOpts) error {
	if opts.AudioBitRate == 0 {
		opts.AudioBitRate = DefaultAudioRate
	} else {
		return validateInArray(opts.AudioBitRate, ValidAudioBitrates[:],
			"invalid audio bitrate: must be one of: 128, 192, 256, 320, or 640 kbps")
	}

	return nil
}

func (f *ffmpeg) validateAccelCodecMatch(videoCodec string) error {
	switch f.AccelMethod {
	case NVENC:
		if videoCodec != "h264_nvenc" {
			return errors.New("nvidia acceleration requires h264_nvenc codec")
		}
	case QSV:
		if videoCodec != "h264_qsv" {
			return errors.New("quicksync acceleration requires h264_qsv codec")
		}
	case VideoToolbox:
		if videoCodec != "h264_videotoolbox" {
			return errors.New("videotoolbox acceleration requires h264_videotoolbox codec")
		}
	case "":
		if videoCodec != "libx264" {
			return errors.New("software encoding requires libx264 codec")
		}
	}

	return nil
}
