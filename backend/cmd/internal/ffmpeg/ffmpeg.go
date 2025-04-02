package ffmpeg

import (
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"os/exec"
	"sync"

	"golang.org/x/sys/unix"
)

type FFmpeg interface {
	CreateHlsStream(opts *HlsOpts) (string, error)
	CancelJob(string) error
	JobsFull(int) bool
}

type ffmpeg struct {
	bin                    string
	HardwareEncoder        string
	EnableHardwareEncoding bool
	jobs                   map[string]job
	mu                     sync.RWMutex
	probe                  ffprobe.Ffprobe
}

var Encoders = map[string]map[string]string{
	"software": {
		"name": "libx264",
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

	probe, err := ffprobe.New(s)
	if err != nil {
		return nil, &ffmpegError{
			Field: "ffprobe",
			Value: "initialization",
			Msg:   fmt.Sprintf("failed to initialize ffprobe: %v", err),
		}
	}

	f := ffmpeg{
		EnableHardwareEncoding: false,
		HardwareEncoder:        Encoders["software"]["name"],
		jobs:                   make(map[string]job),
		probe:                  probe,
	}

	err = unix.Access(path, unix.X_OK)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg at %s is not executable: %w", path, err)
	}

	f.bin = path

	if s.EnableHardwareAcceleration {
		encoder, exists := Encoders[s.HardwareEncoder]
		if exists {
			f.EnableHardwareEncoding = true
			f.HardwareEncoder = encoder["name"]
		} else {
			return nil, fmt.Errorf("invalid hardware encoder: %s", s.HardwareEncoder)
		}
	}

	return &f, nil
}

func NewFFmpeg() *ffmpeg {
	return &ffmpeg{
		bin:                    "ffmpeg",
		jobs:                   make(map[string]job),
		EnableHardwareEncoding: false,
		HardwareEncoder:        "",
	}
}
