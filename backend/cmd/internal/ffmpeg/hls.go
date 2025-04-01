package ffmpeg

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type HlsOpts struct {
	InputPath        string `json:"input_path"`
	OutputDir        string `json:"output_dir"`
	StartTime        int64  `json:"start_time"`
	AudioStreamIndex int    `json:"audio_stream_index"`
	AudioCodec       string `json:"audio_codec"`
	AudioBitRate     int    `json:"audio_bit_rate"`
	AudioChannels    int    `json:"audio_channels"`
	VideoStreamIndex int    `json:"video_stream_index"`
	VideoCodec       string
	VideoBitrate     int    `json:"video_bitrate"`
	VideoHeight      int    `json:"video_height"`
	VideoProfile     string `json:"video_profile"`
	Preset           string `json:"preset"`
	SegmentsUrl      string `json:"segments_url"`
	TotalDuration    int64  `json:"total_duration"`
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

	err = f.createMasterPlaylist(opts)
	if err != nil {
		return "", &ffmpegError{
			Field: "master_playlist",
			Value: opts.OutputDir,
			Msg:   fmt.Sprintf("failed to create master playlist: %v", err),
		}
	}

	cmd := f.prepareHlsCmd(opts)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", &ffmpegError{
			Field: "stderr_pipe",
			Value: "ffmpeg",
			Msg:   fmt.Sprintf("failed to create stderr pipe: %v", err),
		}
	}

	err = cmd.Start()
	if err != nil {
		stderr.Close()
		return "", &ffmpegError{
			Field: "process",
			Value: "ffmpeg",
			Msg:   fmt.Sprintf("failed to start ffmpeg command: %v", err),
		}
	}

	jobID := uuid.New().String()

	cleanup := func() {
		stderr.Close()

		if cmd.Process != nil {
			cmd.Process.Kill()
		}

		f.mu.Lock()

		delete(f.jobs, jobID)

		f.mu.Unlock()
	}

	go func() {
		defer stderr.Close()

		buf := make([]byte, 1024)

		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Printf("failed to read stderr: %v\n", err)
				}

				break
			}

			fmt.Printf("ffmpeg stderr: %s", string(buf[:n]))
		}
	}()

	f.mu.Lock()

	f.jobs[jobID] = job{
		process:   cmd,
		startTime: time.Now(),
		pid:       jobID,
		opts:      opts,
		cleanup:   cleanup,
	}

	f.mu.Unlock()

	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("ffmpeg process error: %v\n", err)
		}

		cleanup()
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

	if opts.InputPath == "" {
		return &ffmpegError{
			Field: "input_path",
			Value: "",
			Msg:   "input path is required",
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

	if opts.OutputDir == "" {
		return &ffmpegError{
			Field: "output_dir",
			Value: "",
			Msg:   "output directory is required",
		}
	}

	err = os.MkdirAll(opts.OutputDir, 0755)
	if err != nil {
		return &ffmpegError{
			Field: "output_dir",
			Value: opts.OutputDir,
			Msg:   fmt.Sprintf("failed to create output directory: %v", err),
		}
	}

	if opts.SegmentsUrl == "" {
		return &ffmpegError{
			Field: "segments_url",
			Value: "",
			Msg:   "segments URL is required",
		}
	}

	err = validateAudioSettings(opts)
	if err != nil {
		return err
	}

	err = validateVideoSettings(opts, f.EnableHardwareEncoding)
	if err != nil {
		return err
	}

	return nil
}

func (f *ffmpeg) prepareHlsCmd(opts *HlsOpts) *exec.Cmd {
	cmdArgs := f.buildBaseArgs(opts)
	cmdArgs = append(cmdArgs, f.buildInputArgs(opts)...)
	cmdArgs = append(cmdArgs, f.buildCodecArgs(opts)...)
	cmdArgs = append(cmdArgs, f.buildOutputArgs(opts)...)

	return exec.Command(f.bin, cmdArgs...)
}

func (f *ffmpeg) buildBaseArgs(opts *HlsOpts) []string {
	args := []string{
		"-y",
		"-hwaccel", "auto",
	}

	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%d", opts.StartTime))
	}

	return args
}

func (f *ffmpeg) buildInputArgs(opts *HlsOpts) []string {
	return []string{
		"-re",
		"-fflags", "+genpts+igndts+ignidx+discardcorrupt+fastseek",
		"-i", opts.InputPath,
		"-sn",
	}
}

func (f *ffmpeg) buildCodecArgs(opts *HlsOpts) []string {
	args := make([]string, 0)

	if opts.VideoCodec == VideoCodecCopy && opts.AudioCodec == AudioCodecCopy {
		return append(args, "-c", "copy")
	}

	args = append(args, "-c:a", opts.AudioCodec)
	if opts.AudioCodec != AudioCodecCopy {
		args = append(args,
			"-b:a", fmt.Sprintf("%dk", opts.AudioBitRate),
			"-ac", fmt.Sprintf("%d", opts.AudioChannels),
			"-map", fmt.Sprintf("0:%d", opts.AudioStreamIndex),
		)
	}

	if opts.VideoCodec == VideoCodecCopy {
		args = append(args, "-c:v", VideoCodecCopy)
	} else {
		args = append(args, f.buildVideoCodecArgs(opts)...)
	}

	return args
}

func (f *ffmpeg) buildVideoCodecArgs(opts *HlsOpts) []string {
	args := make([]string, 0)

	if f.EnableHardwareEncoding {
		args = append(args, "-c:v", Encoders[f.HardwareEncoder]["h264"])
		args = append(args, f.getHardwareEncoderArgs()...)
	} else {
		args = append(args,
			"-c:v", DefaultVideoCodec,
			"-preset", opts.Preset,
			"-crf", "23",
		)
	}

	videoFilter := fmt.Sprintf("scale=-2:%d", opts.VideoHeight)
	args = append(args,
		"-b:v", fmt.Sprintf("%dk", opts.VideoBitrate),
		"-vf", videoFilter,
		"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex),
	)

	if opts.VideoProfile != "" {
		args = append(args, "-profile:v", opts.VideoProfile)
	}

	return args
}

func (f *ffmpeg) getHardwareEncoderArgs() []string {
	switch f.HardwareEncoder {
	case "nvenc":
		return []string{
			"-preset", "p4",
			"-rc", "vbr",
			"-cq", "19",
			"-spatial_aq", "1",
			"-temporal_aq", "1",
		}
	case "qsv":
		return []string{
			"-preset", "medium",
			"-global_quality", "19",
			"-look_ahead", "1",
		}
	case "videotoolbox":
		return []string{
			"-preset", "medium",
			"-allow_sw", "1",
			"-realtime", "0",
		}
	default:
		return nil
	}
}

func (f *ffmpeg) buildOutputArgs(opts *HlsOpts) []string {
	return []string{
		"-f", "hls",
		"-hls_time", DefaultHlsTime,
		"-hls_playlist_type", DefaultPlaylistType,
		"-hls_segment_type", DefaultSegmentType,
		"-hls_segment_filename", filepath.Join(opts.OutputDir, DefaultSegmentPattern),
		"-hls_fmp4_init_filename", DefaultInitFileName,
		"-hls_base_url", opts.SegmentsUrl + "/",
		"-hls_flags", DefaultHlsFlags,
		"-hls_list_size", DefaultHlsListSize,
		"-hls_allow_cache", "0",
		"-master_pl_name", DefaultMasterPlaylist,
		"-hls_init_time", "0",
		filepath.Join(opts.OutputDir, DefaultPlaylistName),
	}
}
