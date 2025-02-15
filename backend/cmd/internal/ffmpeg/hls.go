package ffmpeg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type HlsOpts struct {
	InputPath        string `json:"input_path"`
	OutputDir        string `json:"output_dir"`
	AudioStreamIndex int    `json:"audio_stream_index"`
	AudioCodec       string `json:"audio_codec"`
	AudioBitRate     int    `json:"audio_bit_rate"`
	AudioChannels    int    `json:"audio_channels"`
	VideoStreamIndex int    `json:"video_stream_index"`
	VideoCodec       string `json:"video_codec"`
	VideoBitrate     int    `json:"video_bitrate"`
	VideoHeight      int    `json:"video_height"`
	VideoProfile     string `json:"video_profile"`
	Preset           string `json:"preset"`
}

const (
	DefaultHlsTime        = "5"
	DefaultHlsListSize    = "0"
	DefaultSegmentType    = "fmp4"
	DefaultHlsFlags       = "independent_segments"
	DefaultPlaylistType   = "event"
	DefaultSegmentPattern = "segment%d.m4s"
	DefaultPlaylistName   = "playlist.m3u8"
	DefaultInitFileName   = "init.mp4"
)

func (f *ffmpeg) CreateHlsStream(opts *HlsOpts) error {
	err := f.validateHlsOpts(opts)
	if err != nil {
		return err
	}

	cmd := f.prepareHlsCmd(opts)

	return cmd.Run()
}

func (f *ffmpeg) validateHlsOpts(opts *HlsOpts) error {
	_, err := os.Stat(opts.InputPath)
	if err != nil {
		return fmt.Errorf("input path error: %w", err)
	}

	err = os.MkdirAll(opts.OutputDir, 0755)
	if err != nil {
		return fmt.Errorf("output directory error: %w", err)
	}

	if opts.AudioStreamIndex < 0 {
		return errors.New("invalid audio stream index: must be 0 or greater")
	}

	if opts.VideoStreamIndex < 0 {
		return errors.New("invalid video stream index: must be 0 or greater")
	}

	err = validateAudioSettings(opts)
	if err != nil {
		return err
	}

	err = validateVideoSettings(opts, f.AccelMethod)
	if err != nil {
		return err
	}

	return nil
}

func (f *ffmpeg) prepareHlsCmd(opts *HlsOpts) *exec.Cmd {
	cmdArgs := []string{
		"-i", opts.InputPath,
		"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex),
		"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex),
	}

	if opts.AudioCodec == "copy" {
		cmdArgs = append(cmdArgs, "-c:a", "copy")
	} else if opts.AudioCodec != "" {
		cmdArgs = append(cmdArgs, "-c:a", opts.AudioCodec)
		cmdArgs = append(cmdArgs, "-b:a", fmt.Sprintf("%dk", opts.AudioBitRate))
		cmdArgs = append(cmdArgs, "-ac", fmt.Sprintf("%d", opts.AudioChannels))
	}

	if opts.VideoCodec == "copy" {
		cmdArgs = append(cmdArgs, "-c:v", "copy")
	} else {
		var videoCodec string

		switch f.AccelMethod {
		case NVENC:
			videoCodec = NvencVideoCodec
		case QSV:
			videoCodec = QsvVideoCodec
		case VideoToolbox:
			videoCodec = VtVideoCodec
		default:
			videoCodec = DefaultVideoCodec
		}
		cmdArgs = append(cmdArgs, "-c:v", videoCodec)

		cmdArgs = append(cmdArgs,
			"-b:v", fmt.Sprintf("%dk", opts.VideoBitrate),
			"-vf", fmt.Sprintf("scale=-2:%d", opts.VideoHeight),
			"-preset", opts.Preset,
		)

		if opts.VideoProfile != "" {
			cmdArgs = append(cmdArgs, "-profile:v", opts.VideoProfile)
		}
	}

	cmdArgs = append(cmdArgs,
		"-hls_playlist_type", DefaultPlaylistType,
		"-hls_time", DefaultHlsTime,
		"-hls_list_size", DefaultHlsListSize,
		"-hls_segment_type", DefaultSegmentType,
		"-hls_flags", DefaultHlsFlags+"+program_date_time",
		"-hls_fmp4_init_filename", DefaultInitFileName,
		"-hls_segment_filename", DefaultSegmentPattern,
		filepath.Join(opts.OutputDir, DefaultPlaylistName),
	)

	cmd := exec.Command(f.Bin, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}
