package ffmpeg

import (
	"errors"
	"fmt"
	"os"
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
}

type Settings struct {
	FfmpegPath                 string
	EnableHardwareAcceleration bool
	HardwareEncoder            string
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

func New(s *Settings) (FFmpeg, error) {
	f := ffmpeg{
		EnableHardwareEncoding: false,
		HardwareEncoder:        Encoders["software"]["name"],
		jobs:                   make(map[string]job),
	}

	if s == nil {
		return nil, errors.New("provided nil value for settings in ffmpeg package")
	}

	if s.FfmpegPath == "" {
		return nil, errors.New("ffmpeg path is empty")
	}

	path, err := exec.LookPath(s.FfmpegPath)
	if err != nil {
		_, err = os.Stat(s.FfmpegPath)
		if err != nil {
			return nil, fmt.Errorf("ffmpeg not found at %s: %w", s.FfmpegPath, err)
		}

		path = s.FfmpegPath
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
