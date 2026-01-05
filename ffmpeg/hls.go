package ffmpeg

import (
	"context"
	"fmt"
	"igloo/cmd/internal/helpers"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

type HlsOpts struct {
	InputPath        string `json:"input_path"`
	OutputDir        string `json:"output_dir"`
	StartTime        int64  `json:"start_time"`
	AudioStreamIndex int    `json:"audio_stream_index"`
	AudioCodec       string `json:"audio_codec"`
	AudioChannels    int    `json:"audio_channels"`
	VideoStreamIndex int    `json:"video_stream_index"`
	VideoCodec       string `json:"video_codec"`
	Resolution       int    `json:"resolution"`
	Preset           string `json:"preset"`
	SegmentsUrl      string `json:"segments_url"`
}

func (f *ffmpeg) CreateHlsStream(opts *HlsOpts) (string, error) {
	err := f.validateHlsOpts(opts)
	if err != nil {
		return "", &ffmpegError{
			Field: "validation",
			Value: "opts",
			Msg:   fmt.Sprintf("validation failed: %v", err),
		}
	}

	f.mu.Lock()
	if len(f.jobs) >= 10 {
		f.mu.Unlock()
		return "", &ffmpegError{
			Field: "jobs",
			Value: "max_concurrent",
			Msg:   "maximum number of concurrent jobs (10) reached",
		}
	}
	f.mu.Unlock()

	cmd := f.prepareHlsCmd(opts)
	jobID := uuid.New().String()
	ffmpegCtx, cancelJob := context.WithCancel(context.Background())

	f.mu.Lock()
	f.jobs[jobID] = job{
		ctx:    ffmpegCtx,
		cancel: cancelJob,
	}
	f.mu.Unlock()

	go func() {
		defer func() {
			f.mu.Lock()
			delete(f.jobs, jobID)
			f.mu.Unlock()
			cancelJob()
		}()

		err = cmd.RunWithContext(ffmpegCtx)
		if err != nil {
			fmt.Printf("error running ffmpeg: %v", err)
			return
		}
	}()

	return jobID, nil
}

func (f *ffmpeg) validateHlsOpts(opts *HlsOpts) error {
	if opts == nil {
		return &ffmpegError{
			Field: "opts",
			Value: "nil",
			Msg:   "HLS options cannot be nil",
		}
	}

	err := helpers.CreateDir(opts.OutputDir)
	if err != nil {
		return &ffmpegError{
			Field: "output_dir",
			Value: opts.OutputDir,
			Msg:   fmt.Sprintf("unable to locate output directory: %v", err),
		}
	}

	if opts.SegmentsUrl == "" {
		return &ffmpegError{
			Field: "segments_url",
			Value: "",
			Msg:   "segments URL is required",
		}
	}

	if opts.AudioCodec != AudioCodecCopy {
		err = f.validateAudioSettings(&audioSettings{Codec: opts.AudioCodec, Channels: opts.AudioChannels})
		if err != nil {
			return err
		}
	}

	if opts.VideoCodec != VideoCodecCopy {
		err = f.validateVideoSettings(&videoSettings{Resolution: opts.Resolution, Codec: opts.VideoCodec, Preset: opts.Preset})
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *ffmpeg) prepareHlsCmd(opts *HlsOpts) *exec.Cmd {
	cmdArgs := []string{
		"-y",
		"-hwaccel", "auto",
		"-re",
		"-fflags", "+genpts+igndts+ignidx+discardcorrupt+fastseek",
	}

	if opts.StartTime > 0 {
		cmdArgs = append(cmdArgs, "-itsoffset", fmt.Sprintf("%d", opts.StartTime))
	}

	cmdArgs = append(cmdArgs, "-i", opts.InputPath)

	// Add stream mapping first
	cmdArgs = append(cmdArgs,
		"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex),
		"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex),
	)

	if opts.AudioCodec == "copy" && opts.VideoCodec == AudioCodecCopy {
		cmdArgs = append(cmdArgs, "-c", AudioCodecCopy)
	} else {
		cmdArgs = append(cmdArgs, "-c:a", opts.AudioCodec)

		if opts.AudioCodec != AudioCodecCopy {
			cmdArgs = append(cmdArgs,
				"-ac", fmt.Sprintf("%d", opts.AudioChannels),
				"-b:a", "192k",
			)
		}

		cmdArgs = append(cmdArgs,
			"-c:v", opts.VideoCodec,
			"-vf", fmt.Sprintf("scale=-2:%d", opts.Resolution),
		)

		if f.En
	}

	cmdArgs = append(cmdArgs,
		"-hls_time", fmt.Sprintf("%d", DefaultHlsTime),
		"-hls_playlist_type", DefaultPlaylistType,
		"-hls_segment_type", DefaultSegmentType,
		"-hls_segment_filename", filepath.Join(opts.OutputDir, DefaultSegmentPattern),
		"-hls_fmp4_init_filename", DefaultInitFileName,
		"-hls_flags", DefaultHlsFlags,
		"-hls_list_size", DefaultHlsListSize,
		"-hls_base_url", opts.SegmentsUrl,
	)

	return exec.Command(f.bin, cmdArgs...)
}
