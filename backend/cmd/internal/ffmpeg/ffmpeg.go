package ffmpeg

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
)

type FFmpeg interface {
	CreateHlsStream(opts *HlsOpts) error
}

type ffmpeg struct {
	Bin         string
	AccelMethod string
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

	if ffmpegPath != "ffmpeg" && !filepath.IsAbs(ffmpegPath) {
		return nil, errors.New("ffmpeg path must be absolute unless using 'ffmpeg' from PATH")
	}

	path, err := exec.LookPath(ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found or not executable: %w", err)
	}

	f := &ffmpeg{
		Bin:         path,
		AccelMethod: NoAccel,
	}

	return f, nil
}
