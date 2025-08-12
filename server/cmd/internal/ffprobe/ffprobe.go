package ffprobe

import (
	"errors"
	"fmt"
	"os/exec"
)

func New(ffprobePath string) (FfprobeInterface, error) {
	if ffprobePath == "" {
		return nil, errors.New("ffprobe path is empty")
	}

	var f Ffprobe

	path, err := exec.LookPath(ffprobePath)
	if err != nil {
		return nil, fmt.Errorf("unable to find ffprobe at %s: %w", ffprobePath, err)
	}

	_, err = exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("ffprobe not found or not executable at %s: %w", path, err)
	}

	f.bin = ffprobePath

	return &f, nil
}
