package ffmpeg

import (
	"fmt"
	"igloo/cmd/internal/database"
	"os/exec"
	"sync"

	"golang.org/x/sys/unix"
)

type FFmpeg interface {
	JobsFull() bool
	CreateHlsStream(opts *HlsOpts) (string, error)
	CancelJob(jobID string) error
}

type ffmpeg struct {
	bin                        string
	enableHardwareAcceleration bool
	encoder                    map[string]string
	jobs                       map[string]job
	mu                         sync.RWMutex
	maxJobs                    int
}

var Encoders = map[string]map[string]string{
	"software": {
		"name": "software",
		"h264": "libx264",
		"h265": "libx265",
	},
	"nvidia": {
		"name": "nvenc",
		"h264": "h264_nvenc",
		"h265": "hevc_nvenc",
	},
	"intel": {
		"name": "qsv",
		"h264": "h264_qsv",
		"h265": "hevc_qsv",
	},
	"mac": {
		"name": "videotoolbox",
		"h264": "h264_videotoolbox",
		"h265": "hevc_videotoolbox",
	},
}

func New(s *database.GlobalSetting) (FFmpeg, error) {
	if s == nil {
		return nil, &ffmpegError{
			Field: "settings",
			Value: "nil",
			Msg:   "settings cannot be nil",
		}
	}

	f := ffmpeg{
		bin:                        s.FfmpegPath,
		enableHardwareAcceleration: false,
		encoder:                    Encoders["software"],
		jobs:                       make(map[string]job),
		maxJobs:                    10,
	}

	if s.FfmpegPath == "" {
		return nil, &ffmpegError{
			Field: "ffmpeg_path",
			Value: "",
			Msg:   "ffmpeg path is required",
		}
	}

	path, err := exec.LookPath(s.FfmpegPath)
	if err != nil {
		return nil, &ffmpegError{
			Field: "ffmpeg_path",
			Value: s.FfmpegPath,
			Msg:   fmt.Sprintf("unable to find ffmpeg at %s: %v", s.FfmpegPath, err),
		}
	}

	err = unix.Access(path, unix.X_OK)
	if err != nil {
		return nil, &ffmpegError{
			Field: "ffmpeg_path",
			Value: s.FfmpegPath,
			Msg:   fmt.Sprintf("unable to access ffmpeg at %s: %v", s.FfmpegPath, err),
		}
	}

	if s.EnableHardwareAcceleration {
		encoder, exists := Encoders[s.HardwareEncoder]
		if exists {
			f.enableHardwareAcceleration = true
			f.encoder = encoder
		} else {
			return nil, fmt.Errorf("invalid hardware encoder: %s", s.HardwareEncoder)
		}
	}

	return &f, nil
}
