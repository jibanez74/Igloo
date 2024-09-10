package models

import (
	"errors"
	"os"
)

type TranscodeVideoOpts struct {
	FilePath      string
	OutputName    string
	OutputDir     string
	Container     string
	VideoCodec    string
	AudioCodec    string
	AudioChannels float64
	Resolution    uint
}

func (t *TranscodeVideoOpts) ValidateValues() error {
	if t.FilePath == "" {
		return errors.New("FilePath cannot be empty")
	}

	if t.OutputName == "" {
		return errors.New("output name is required")
	}

	if t.OutputDir == "" {
		return errors.New("output directory is required")
	}

	if t.Container == "" {
		return errors.New("container cannot be empty")
	}

	if t.VideoCodec == "" {
		return errors.New("VideoCodec cannot be empty")
	}

	if t.AudioCodec == "" {
		return errors.New("AudioCodec cannot be empty")
	}

	_, err := os.Stat(t.FilePath)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("file does not exist at the specified FilePath")
	}

	validResolutions := []uint{480, 720, 1080, 2160}
	isValidResolution := false

	for _, res := range validResolutions {
		if t.Resolution == res {
			isValidResolution = true
			break
		}
	}

	if !isValidResolution {
		return errors.New("invalid Resolution. Must be 480, 720, 1080, or 2160")
	}

	return nil
}
