package ffmpeg

import (
	"errors"
	"path/filepath"
)

// validateInArray checks if a value exists in a slice of valid values
// Used for validating various ffmpeg settings like codecs, bitrates, etc.
func validateInArray[T comparable](value T, validValues []T, errMsg string) error {
	for _, v := range validValues {
		if value == v {
			return nil
		}
	}

	return errors.New(errMsg)
}

// validateFfmpegPath checks if the provided path is valid
func validateFfmpegPath(path string) error {
	if path == "" {
		return errors.New("ffmpeg path is required")
	}

	if path != "ffmpeg" && !filepath.IsAbs(path) {
		return errors.New("ffmpeg path must be absolute unless using 'ffmpeg' from PATH")
	}

	return nil
}

// validateAccelMethod checks if the acceleration method is supported
func validateAccelMethod(accel string) error {
	switch accel {
	case NoAccel, NVENC, QSV, VideoToolbox:
		return nil
	default:
		return errors.New("unsupported acceleration method: must be one of: nvenc, qsv, videotoolbox, or empty string for software encoding")
	}
}
