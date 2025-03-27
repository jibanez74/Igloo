package ffmpeg

import (
	"fmt"
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
	HlsTagProgramDateTime     = "#EXT-X-PROGRAM-DATE-TIME:2024-01-01T00:00:00Z"
	HlsTagMap                 = "#EXT-X-MAP:URI=\"%s\",BANDWIDTH=0"
)

func (f *ffmpeg) CreateHlsStream(opts *HlsOpts) (string, error) {
	err := f.validateHlsOpts(opts)
	if err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	err = f.createVodPlaylist(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create VOD playlist: %w", err)
	}

	cmd := f.prepareHlsCmd(opts)

	err = cmd.Start()
	if err != nil {
		return "", fmt.Errorf("failed to start ffmpeg command: %w", err)
	}

	pid := uuid.NewString()

	f.mu.Lock()
	f.jobs[pid] = job{
		process:   cmd,
		startTime: time.Now(),
		status:    "running",
	}
	f.mu.Unlock()

	go func() {
		err = cmd.Wait()
		if err != nil {
			f.mu.Lock()
			delete(f.jobs, pid)
			f.mu.Unlock()
		}
	}()

	return pid, nil
}

func (f *ffmpeg) validateHlsOpts(opts *HlsOpts) error {
	_, err := os.Stat(opts.InputPath)
	if err != nil {
		return fmt.Errorf("unable to locate input file: %w", err)
	}

	err = os.MkdirAll(opts.OutputDir, 0755)
	if err != nil {
		return fmt.Errorf("output directory error: %w", err)
	}

	validateAudioSettings(opts)

	validateVideoSettings(opts, f.AccelMethod)

	return nil
}

func (f *ffmpeg) createVodPlaylist(opts *HlsOpts) error {
	playlistPath := filepath.Join(opts.OutputDir, DefaultPlaylistName)

	file, err := os.Create(playlistPath)
	if err != nil {
		return fmt.Errorf("failed to create playlist file at %s: %w", playlistPath, err)
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s\n", HlsTagExtM3U)
	if err != nil {
		return fmt.Errorf("failed to write EXTM3U tag: %w", err)
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagVersion)
	if err != nil {
		return fmt.Errorf("failed to write version tag: %w", err)
	}

	_, err = fmt.Fprintf(file, HlsTagTargetDuration+"\n", DefaultHlsTime)
	if err != nil {
		return fmt.Errorf("failed to write target duration tag: %w", err)
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagMediaSequence)
	if err != nil {
		return fmt.Errorf("failed to write media sequence tag: %w", err)
	}

	_, err = fmt.Fprintf(file, HlsTagPlaylistType+"\n", DefaultPlaylistType)
	if err != nil {
		return fmt.Errorf("failed to write playlist type tag: %w", err)
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagIndependentSegments)
	if err != nil {
		return fmt.Errorf("failed to write independent segments tag: %w", err)
	}

	_, err = fmt.Fprintf(file, "%s\n", HlsTagProgramDateTime)
	if err != nil {
		return fmt.Errorf("failed to write program date time tag: %w", err)
	}

	_, err = fmt.Fprintf(file, HlsTagMap+"\n", DefaultInitFileName)
	if err != nil {
		return fmt.Errorf("failed to write map tag: %w", err)
	}

	return nil
}

func (f *ffmpeg) prepareHlsCmd(opts *HlsOpts) *exec.Cmd {
	cmdArgs := []string{
		"ss", fmt.Sprintf("%d", opts.StartTime),
		"-re",
		"-fflags", "+genpts+igndts+ignidx+discardcorrupt+fastseek",
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
				// "-g", "120",
				// "-keyint_min", "120",
				// "-force_key_frames", "expr:gte(t,n_forced*2)",
				"-map", fmt.Sprintf("0:%d", opts.VideoStreamIndex),
			)

			if opts.VideoProfile != "" {
				cmdArgs = append(cmdArgs, "-profile:v", opts.VideoProfile)
			}
		}
	}

	cmdArgs = append(cmdArgs,
		"-hls_playlist_type", DefaultPlaylistType,
		"-hls_time", DefaultHlsTime,
		"-hls_list_size", DefaultHlsListSize,
		"-hls_segment_type", DefaultSegmentType,
		"-hls_flags", DefaultHlsFlags,
		"-hls_fmp4_init_filename", DefaultInitFileName,
		"-hls_segment_filename", filepath.Join(opts.OutputDir, DefaultSegmentPattern),
		"-hls_base_url", opts.SegmentsUrl+"/",
		filepath.Join(opts.OutputDir, DefaultPlaylistName),
	)

	return exec.Command(f.Bin, cmdArgs...)
}
