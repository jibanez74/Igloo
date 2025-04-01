package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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

	width := opts.VideoHeight * 16 / 9

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

func (f *ffmpeg) createMasterPlaylist(opts *HlsOpts) error {
	playlistPath := filepath.Join(opts.OutputDir, DefaultMasterPlaylist)

	file, err := os.Create(playlistPath)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_path",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to create master playlist file: %v", err),
		}
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s\n", HlsTagExtM3U)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagExtM3U,
			Msg:   fmt.Sprintf("failed to write EXTM3U tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagVersion)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagVersion,
			Msg:   fmt.Sprintf("failed to write version tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, HlsTagTargetDuration+"\n", DefaultHlsTime)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagTargetDuration,
			Msg:   fmt.Sprintf("failed to write target duration tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagMediaSequence)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagMediaSequence,
			Msg:   fmt.Sprintf("failed to write media sequence tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, HlsTagPlaylistType+"\n", DefaultPlaylistType)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
			Value: HlsTagPlaylistType,
			Msg:   fmt.Sprintf("failed to write playlist type tag: %v", err),
		}
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagIndependentSegments)
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_header",
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

	_, err = fmt.Fprintf(file, HlsTagEndList+"\n")
	if err != nil {
		return &ffmpegError{
			Field: "master_playlist_footer",
			Value: HlsTagEndList,
			Msg:   fmt.Sprintf("failed to write end list tag: %v", err),
		}
	}

	return nil
}
