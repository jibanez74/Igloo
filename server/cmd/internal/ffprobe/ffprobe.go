package ffprobe

import (
	"fmt"
	"os"
	"sync"
)

type FfprobeInterface interface {
	GetMetadata(filePath string) (*FfprobeResult, error)
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
// embed mode extracts to a temp directory, systembin mode returns a fixed path.
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

	err := os.RemoveAll(extractedDir)
	if err != nil {
		return fmt.Errorf("failed to cleanup ffprobe: %w", err)
	}

	extractedDir = ""
	instance = nil

	return nil
}
