package ffmpeg

import (
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"os/exec"
	"sync"
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
	f := ffmpeg{
		EnableHardwareEncoding: false,
		HardwareEncoder:        Encoders["software"]["name"],
	}

	if s == nil {
		return nil, errors.New("provided nil value for settings in ffmpeg package")
	}

	if s.FfmpegPath == "" {
		return nil, errors.New("ffmpeg path is empty")
	}

	path, err := exec.LookPath(s.FfmpegPath)
	if err != nil {
		return nil, fmt.Errorf("unable to find ffmpeg at %s: %w", s.FfmpegPath, err)
	}

	_, err = exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found or not executable at %s: %w", path, err)
	}

	f.bin = s.FfmpegPath

	if s.EnableHardwareAcceleration {
		_, exists := Encoders[s.HardwareEncoder]
		if exists {
			f.EnableHardwareEncoding = true
			f.HardwareEncoder = Encoders[s.HardwareEncoder]["name"]
		}
	}

	return &f, nil
}
