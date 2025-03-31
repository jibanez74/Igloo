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
}

const (
	DefaultHlsTime            = "6"
	DefaultHlsListSize        = "0"
	DefaultSegmentType        = "fmp4"
	DefaultHlsFlags           = "independent_segments+program_date_time+append_list+discont_start"
	DefaultPlaylistType       = "vod"
	DefaultSegmentPattern     = "segment_%d.m4s"
	DefaultPlaylistName       = "playlist.m3u8"
	DefaultInitFileName       = "init.mp4"
	HlsTagExtM3U              = "#EXTM3U"
	HlsTagVersion             = "#EXT-X-VERSION:7"
	HlsTagTargetDuration      = "#EXT-X-TARGETDURATION:%s"
	HlsTagMediaSequence       = "#EXT-X-MEDIA-SEQUENCE:0"
	HlsTagPlaylistType        = "#EXT-X-PLAYLIST-TYPE:%s"
	HlsTagIndependentSegments = "#EXT-X-INDEPENDENT-SEGMENTS"
	HlsTagProgramDateTime     = "#EXT-X-PROGRAM-DATE-TIME:%s"
	HlsTagMap                 = "#EXT-X-MAP:URI=\"%s\",BANDWIDTH=0"
	HlsTagStreamInf           = "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",FRAME-RATE=%s"
	HlsTagStreamInfAudio      = "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",FRAME-RATE=%s,AUDIO=\"audio\""
	HlsTagMedia               = "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",DEFAULT=YES,URI=\"%s\",LANGUAGE=\"%s\",CODECS=\"%s\",CHANNELS=\"%d\""
)

func (f *ffmpeg) CreateHlsStream(opts *HlsOpts) (string, error) {
	err := f.validateHlsOpts(opts)
	if err != nil {
		return "", &ffmpegError{
			Field: "validation",
			Value: "opts",
			Msg:   fmt.Sprintf("validation failed: %v", err),
		}
	}

	err = f.createVodPlaylist(opts)
	if err != nil {
		return "", &ffmpegError{
			Field: "playlist",
			Value: opts.OutputDir,
			Msg:   fmt.Sprintf("failed to create VOD playlist: %v", err),
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
		filepath.Join(opts.OutputDir, DefaultPlaylistName),
	}
}

func (f *ffmpeg) createVodPlaylist(opts *HlsOpts) error {
	playlistPath := filepath.Join(opts.OutputDir, DefaultPlaylistName)

	file, err := os.Create(playlistPath)
	if err != nil {
		return &ffmpegError{
			Field: "playlist_path",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to create playlist file: %v", err),
		}
	}
	defer file.Close()

	err = writeHlsHeader(file, opts)
	if err != nil {
		return err
	}

	if opts.AudioCodec != AudioCodecCopy {
		err = writeAudioMediaTag(file, opts)
		if err != nil {
			return err
		}
	}

	err = writeStreamInfoTag(file, opts)
	if err != nil {
		return err
	}

	err = writeInitSegmentTag(file)
	if err != nil {
		return err
	}

	return nil
}

func writeHlsHeader(file *os.File, opts *HlsOpts) error {
	_, err := fmt.Fprintf(file, "%s\n", HlsTagExtM3U)
	if err != nil {
		return &ffmpegError{
			Field: "playlist_header",
			Value: HlsTagExtM3U,
			Msg:   fmt.Sprintf("failed to write EXTM3U tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagVersion)
	if err != nil {
		return &ffmpegError{
			Field: "playlist_header",
			Value: HlsTagVersion,
			Msg:   fmt.Sprintf("failed to write version tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, HlsTagTargetDuration+"\n", DefaultHlsTime)
	if err != nil {
		return &ffmpegError{
			Field: "playlist_header",
			Value: HlsTagTargetDuration,
			Msg:   fmt.Sprintf("failed to write target duration tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagMediaSequence)
	if err != nil {
		return &ffmpegError{
			Field: "playlist_header",
			Value: HlsTagMediaSequence,
			Msg:   fmt.Sprintf("failed to write media sequence tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, HlsTagPlaylistType+"\n", DefaultPlaylistType)
	if err != nil {
		return &ffmpegError{
			Field: "playlist_header",
			Value: HlsTagPlaylistType,
			Msg:   fmt.Sprintf("failed to write playlist type tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagIndependentSegments)
	if err != nil {
		return &ffmpegError{
			Field: "playlist_header",
			Value: HlsTagIndependentSegments,
			Msg:   fmt.Sprintf("failed to write independent segments tag: %v", err),
		}
	}

	startTime := time.Now()
	if opts.StartTime > 0 {
		startTime = startTime.Add(time.Duration(-opts.StartTime) * time.Second)
	}

	programDateTime := startTime.UTC().Format(time.RFC3339)
	_, err = fmt.Fprintf(file, HlsTagProgramDateTime+"\n", programDateTime)
	if err != nil {
		return &ffmpegError{
			Field: "program_date_time",
			Value: programDateTime,
			Msg:   fmt.Sprintf("failed to write program date time: %v", err),
		}
	}

	return nil
}

func writeAudioMediaTag(file *os.File, opts *HlsOpts) error {
	audioCodec := getAudioCodecString(opts.AudioCodec)
	if audioCodec == "" {
		return &ffmpegError{
			Field: "audio_codec",
			Value: opts.AudioCodec,
			Msg:   "unsupported audio codec",
		}
	}

	_, err := fmt.Fprintf(file, HlsTagMedia+"\n",
		"Audio",
		"audio.m3u8",
		"und",
		audioCodec,
		opts.AudioChannels)

	if err != nil {
		return &ffmpegError{
			Field: "audio_media_tag",
			Value: "audio.m3u8",
			Msg:   fmt.Sprintf("failed to write audio media tag: %v", err),
		}
	}

	return nil
}

func writeStreamInfoTag(file *os.File, opts *HlsOpts) error {
	videoCodec := getVideoCodecString(opts.VideoCodec)
	if videoCodec == "" {
		return &ffmpegError{
			Field: "video_codec",
			Value: opts.VideoCodec,
			Msg:   "unsupported video codec",
		}
	}

	totalBitrate := opts.VideoBitrate
	if opts.AudioCodec != AudioCodecCopy {
		totalBitrate += opts.AudioBitRate
	}

	width := opts.VideoHeight * 16 / 9 // Default to 16:9

	var err error

	if opts.AudioCodec == AudioCodecCopy {
		_, err = fmt.Fprintf(file, HlsTagStreamInf+"\n",
			totalBitrate*1000, // Convert to bits per second
			width,
			opts.VideoHeight,
			videoCodec,
			"30.000") // Default frame rate
	} else {
		_, err = fmt.Fprintf(file, HlsTagStreamInfAudio+"\n",
			totalBitrate*1000,
			width,
			opts.VideoHeight,
			videoCodec,
			"30.000")
	}

	if err != nil {
		return &ffmpegError{
			Field: "stream_info_tag",
			Value: fmt.Sprintf("%dx%d", width, opts.VideoHeight),
			Msg:   fmt.Sprintf("failed to write stream info tag: %v", err),
		}
	}

	return nil
}

func writeInitSegmentTag(file *os.File) error {
	_, err := fmt.Fprintf(file, HlsTagMap+"\n", DefaultInitFileName)
	if err != nil {
		return &ffmpegError{
			Field: "init_segment_tag",
			Value: DefaultInitFileName,
			Msg:   fmt.Sprintf("failed to write initialization segment tag: %v", err),
		}
	}

	return nil
}

func getAudioCodecString(codec string) string {
	switch codec {
	case AudioCodecAAC:
		return "mp4a.40.2"
	case AudioCodecAC3:
		return "ac-3"
	case AudioCodecMP3:
		return "mp4a.40.34"
	case AudioCodecFLAC:
		return "flac"
	default:
		return ""
	}
}

func getVideoCodecString(codec string) string {
	switch codec {
	case VideoCodecH264:
		return "avc1.64001f"
	case VideoCodecH265:
		return "hevc.1.6.L93.B0"
	case VideoCodecCopy:
		return "avc1.64001f" // Default to H.264 for copy mode
	default:
		return ""
	}
}
