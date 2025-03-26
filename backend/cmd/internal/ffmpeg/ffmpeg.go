package ffmpeg

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
)

type FFmpeg interface {
	CreateHlsStream(opts *HlsOpts) (string, error)
	CancelJob(string) error
	JobsFull(int) bool
}

type ffmpeg struct {
	Bin         string
	AccelMethod string
	jobs        map[string]job
	mu          sync.RWMutex
}

const (
	NoAccel      = ""
	NVENC        = "nvenc"        // NVIDIA GPU acceleration
	QSV          = "qsv"          // Intel QuickSync
	VideoToolbox = "videotoolbox" // Apple VideoToolbox
)

func New(ffmpegPath string) (FFmpeg, error) {
	if ffmpegPath == "" {
		return nil, errors.New("ffmpeg path is empty")
	}

	path, err := exec.LookPath(ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	_, err = exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found or not executable at %s: %w", path, err)
	}

	f := ffmpeg{
		Bin:         path,
		AccelMethod: NVENC,
		jobs:        make(map[string]job),
	}

	return &f, nil
}
