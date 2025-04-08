package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	fluentffmpeg "github.com/modfy/fluent-ffmpeg"
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

	vodPath := filepath.Join(opts.OutputDir, VodPlaylistName)

	go func() {
		defer func() {
			f.mu.Lock()
			delete(f.jobs, jobID)
			f.mu.Unlock()
			cancelJob()

			// Finalize the VOD playlist when FFmpeg completes
			if err := f.finalizeVODPlaylist(vodPath); err != nil {
				fmt.Printf("error finalizing VOD playlist: %v", err)
			}
		}()

		err = cmd.RunWithContext(ffmpegCtx)
		if err != nil {
			fmt.Printf("error running ffmpeg: %v", err)
		}
	}()

	go func() {
		err = f.monitorAndUpdatePlaylists(ffmpegCtx, opts.OutputDir)
		if err != nil {
			fmt.Printf("error monitoring and updating playlists: %v", err)

			f.mu.Lock()
			job, exists := f.jobs[jobID]
			if exists {
				job.cancel()
				delete(f.jobs, jobID)
			}
			f.mu.Unlock()

			// Finalize the VOD playlist if monitoring fails
			if err := f.finalizeVODPlaylist(vodPath); err != nil {
				fmt.Printf("error finalizing VOD playlist: %v", err)
			}
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

	_, err := os.Stat(opts.InputPath)
	if err != nil {
		return &ffmpegError{
			Field: "input_path",
			Value: opts.InputPath,
			Msg:   fmt.Sprintf("unable to locate input file: %v", err),
		}
	}

	_, err = os.Stat(opts.OutputDir)
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

	if opts.AudioCodec != "copy" {
		err = f.validateAudioSettings(&audioSettings{Codec: opts.AudioCodec, Channels: opts.AudioChannels})
		if err != nil {
			return err
		}
	}

	if opts.VideoCodec != "copy" {
		err = f.validateVideoSettings(&videoSettings{Resolution: opts.Resolution})
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *ffmpeg) prepareHlsCmd(opts *HlsOpts) *fluentffmpeg.Command {
	inputOpts := []string{
		"-y",
		"-hwaccel", "auto",
		"-fflags", "+genpts+igndts+ignidx+discardcorrupt+fastseek",
	}

	outputOpts := []string{
		"-hls_time", fmt.Sprintf("%d", DefaultHlsTime),
		"-hls_playlist_type", DefaultPlaylistType,
		"-hls_segment_type", DefaultSegmentType,
		"-hls_segment_filename", filepath.Join(opts.OutputDir, DefaultSegmentPattern),
		"-hls_fmp4_init_filename", DefaultInitFileName,
		"-hls_flags", DefaultHlsFlags,
		"-hls_list_size", DefaultHlsListSize,
		"-hls_base_url", opts.SegmentsUrl,
	}

	cmd := fluentffmpeg.NewCommand(f.bin).
		InputPath(opts.InputPath).
		NativeFramerateInput(true).
		InputOptions(inputOpts...).
		AudioCodec(opts.AudioCodec).
		OutputFormat("hls").
		OutputOptions(outputOpts...).
		OutputPath(filepath.Join(opts.OutputDir, DefaultPlaylistName))

	if opts.AudioCodec != "copy" {
		cmd.AudioChannels(opts.AudioChannels)
	}

	if opts.VideoCodec == "copy" {
		cmd.VideoCodec(opts.VideoCodec)
	} else {
		cmd.VideoCodec(f.encoder[opts.VideoCodec]).
			Resolution(ValidResolutions[opts.Resolution]).
			Preset(opts.Preset)
	}

	return cmd
}
