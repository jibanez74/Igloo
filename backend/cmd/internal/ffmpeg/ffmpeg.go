package ffmpeg

import (
	"errors"
	"fmt"
	"os/exec"
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
		return nil, errors.New("unable to get ffmpeg path from environment variables")
	}

	f := ffmpeg{
		Bin:         ffmpegPath,
		AccelMethod: NoAccel,
	}

	if f.Bin == "ffmpeg" {
		return &f, nil
	}

	_, err := exec.LookPath(f.Bin)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found or not executable: %w", err)
	}

	return &f, nil
}
