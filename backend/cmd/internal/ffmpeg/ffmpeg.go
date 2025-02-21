package ffmpeg

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
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
	NoAccel      = ""             // Software encoding (CPU)
	NVENC        = "nvenc"        // NVIDIA GPU acceleration
	QSV          = "qsv"          // Intel QuickSync
	VideoToolbox = "videotoolbox" // Apple VideoToolbox
)

func New(ffmpegPath string) (FFmpeg, error) {
	if ffmpegPath == "" {
		return nil, errors.New("ffmpeg path is required")
	}

	f := ffmpeg{
		Bin:         ffmpegPath,
		AccelMethod: NVENC,
		jobs:        make(map[string]job),
	}

	if f.Bin == "ffmpeg" {
		path, err := exec.LookPath(f.Bin)
		if err != nil {
			return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
		}

		f.Bin = path
	} else {
		absPath, err := filepath.Abs(f.Bin)
		if err != nil {
			return nil, fmt.Errorf("invalid ffmpeg path: %w", err)
		}

		f.Bin = absPath

		_, err = exec.LookPath(f.Bin)
		if err != nil {
			return nil, fmt.Errorf("ffmpeg not found or not executable at %s: %w", f.Bin, err)
		}
	}

	return &f, nil
}
