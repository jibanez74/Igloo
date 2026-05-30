package ffprobe

import (
	"sync"

	"igloo/cmd/internal/mediabin"
)

type FfprobeInterface interface {
	GetMetadata(filePath string) (*FfprobeResult, error)
	GetAudioMetadata(filePath string) (*FfprobeResult, error)
}

type ffprobe struct {
	bin string
}

var _ FfprobeInterface = (*ffprobe)(nil)

var (
	instance     *ffprobe
	instanceMu   sync.Mutex
	extractedDir string
)

// New returns a singleton ffprobe instance.
// resolveBinaryPath is defined in build-tag-specific files:
// embedded mode extracts a release payload; externalbin mode uses host ffprobe.
func New() (FfprobeInterface, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	binPath, err := resolveBinaryPath()
	if err != nil {
		return nil, err
	}

	instance = &ffprobe{bin: binPath}

	return instance, nil
}

// Cleanup removes the extracted binary and its temp directory.
// Should be called when the application shuts down.
func Cleanup() error {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if extractedDir == "" {
		instance = nil
		return nil
	}

	if err := mediabin.CleanupExtracted("ffprobe", extractedDir); err != nil {
		return err
	}

	extractedDir = ""
	instance = nil

	return nil
}
