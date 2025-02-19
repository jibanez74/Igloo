package ffmpeg

import (
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
	SegmentsUrl      string `json:"segments_url"`
}

const (
	DefaultHlsTime        = "5"
	DefaultHlsListSize    = "0"
	DefaultSegmentType    = "fmp4"
	DefaultHlsFlags       = "independent_segments+program_date_time+append_list"
	DefaultPlaylistType   = "event"
	DefaultSegmentPattern = "segment_%d.m4s"
	DefaultPlaylistName   = "playlist.m3u8"
	DefaultInitFileName   = "init.mp4"
)

func (f *ffmpeg) CreateHlsStream(opts *HlsOpts) error {
	err := f.validateHlsOpts(opts)
	if err != nil {
		return err
	}

	cmd := f.prepareHlsCmd(opts)

	err = cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
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

	validateAudioSettings(opts)

	validateVideoSettings(opts, f.AccelMethod)

	return nil
}

func (f *ffmpeg) prepareHlsCmd(opts *HlsOpts) *exec.Cmd {
	cmdArgs := []string{
		"-y",
		"-i", opts.InputPath,
		"-sn",
	}

	if opts.VideoCodec == "copy" && opts.AudioCodec == "copy" {
		cmdArgs = append(cmdArgs, "-c", "copy")
	} else {
		cmdArgs = append(cmdArgs, "-c:a", opts.AudioCodec)

		if opts.AudioCodec != "copy" {
			cmdArgs = append(cmdArgs,
				"-b:a", fmt.Sprintf("%dk", opts.AudioBitRate),
				"-ac", fmt.Sprintf("%d", opts.AudioChannels),
				"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex),
			)
		}

		if opts.VideoCodec == "copy" {
			cmdArgs = append(cmdArgs, "-c:v", opts.VideoCodec)
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

			cmdArgs = append(cmdArgs,
				"-c:v", videoCodec,
				"-b:v", fmt.Sprintf("%dk", opts.VideoBitrate),
				"-vf", fmt.Sprintf("scale=-2:%d", opts.VideoHeight),
				"-preset", opts.Preset,
				"-g", "120",
				"-keyint_min", "120",
				"-force_key_frames", "expr:gte(t,n_forced*2)",
				"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex),
			)

			if opts.VideoProfile != "" {
				cmdArgs = append(cmdArgs, "-profile:v", opts.VideoProfile)
			}
		}
	}

	segmentFilename := filepath.Join(opts.OutputDir, DefaultSegmentPattern)
	if opts.SegmentsUrl != "" {
		segmentFilename = filepath.Join(opts.SegmentsUrl, DefaultSegmentPattern)
	}

	cmdArgs = append(cmdArgs,
		"-hls_playlist_type", DefaultPlaylistType,
		"-hls_time", DefaultHlsTime,
		"-hls_list_size", DefaultHlsListSize,
		"-hls_segment_type", DefaultSegmentType,
		"-hls_flags", DefaultHlsFlags,
		"-hls_fmp4_init_filename", DefaultInitFileName,
		"-hls_segment_filename", segmentFilename,
		"-hls_base_url", opts.SegmentsUrl+"/",
		filepath.Join(opts.OutputDir, DefaultPlaylistName),
	)

	return exec.Command(f.Bin, cmdArgs...)
}
