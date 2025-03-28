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

var Encoders = map[string]string{
	"nvidia": "nvenc",
	"intel":  "qsv",
	"mac":    "videotoolbox",
}

func New(s *database.GlobalSetting) (FFmpeg, error) {
	f := ffmpeg{
		EnableHardwareEncoding: false,
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
		encoder, exists := Encoders[s.HardwareEncoder]
		if exists {
			f.EnableHardwareEncoding = true
			f.HardwareEncoder = encoder
		}
	}

	return &f, nil
}
