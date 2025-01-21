package ffmpeg

import (
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

func New(ffmpegPath string, accelMethod string) (FFmpeg, error) {
	err := validateFfmpegPath(ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("invalid ffmpeg path: %w", err)
	}

	path, err := exec.LookPath(ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found or not executable: %w", err)
	}

	f := &ffmpeg{
		Bin: path,
	}

	err = validateAccelMethod(accelMethod)
	if err != nil {
		return nil, fmt.Errorf("invalid acceleration method: %w", err)
	}

	f.AccelMethod = accelMethod

	return f, nil
}
