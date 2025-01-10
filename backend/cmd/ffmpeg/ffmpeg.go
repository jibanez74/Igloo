package ffmpeg

import (
	"errors"
	"os/exec"
	"path/filepath"
)

type FFmpeg interface {
	CreateHlsStream(opts *HlsOpts) error
	GetAccelMethod() string
	validateAccelMethod(accel string) error
	setAcceleration(accel string) error
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
	var f ffmpeg

	if ffmpegPath == "" {
		return nil, errors.New("ffmpeg binary path is required")
	}

	if ffmpegPath != "ffmpeg" {
		if !filepath.IsAbs(ffmpegPath) {
			return nil, errors.New("ffmpeg path must be absolute")
		}
	}

	_, err := exec.LookPath(ffmpegPath)
	if err != nil {
		return nil, errors.New("ffmpeg binary not found or not executable")
	}

	f.Bin = ffmpegPath

	err = f.setAcceleration(accelMethod)
	if err != nil {
		return nil, err
	}

	return &f, nil
}

func (f *ffmpeg) validateAccelMethod(accel string) error {
	switch accel {
	case NoAccel, NVENC, QSV, VideoToolbox:
		return nil
	default:
		return errors.New("unsupported acceleration method: must be one of: nvenc, qsv, videotoolbox, or empty string for software encoding")
	}
}

func (f *ffmpeg) setAcceleration(accel string) error {
	err := f.validateAccelMethod(accel)
	if err != nil {
		return err
	}

	f.AccelMethod = accel

	return nil
}

func (f *ffmpeg) GetAccelMethod() string {
	return f.AccelMethod
}
