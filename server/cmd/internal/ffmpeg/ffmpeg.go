package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type FFmpegInterface interface {
	RunHLS(ctx context.Context, params HLSParams, onExit func(exitErr error, stderrTail []string)) (*exec.Cmd, error)
	ExtractSubtitleAsWebVTT(ctx context.Context, sourcePath string, streamIndex int64) ([]byte, error)
}

type ffmpeg struct {
	bin string
}

var _ FFmpegInterface = (*ffmpeg)(nil)

var (
	instance     *ffmpeg
	instanceMu   sync.Mutex
	extractedDir string
)

// New returns a singleton FFmpeg implementation.
// resolveBinaryPath is defined in build-tag-specific files:
// embed mode extracts to a temp directory, systembin mode returns a fixed path.
func New() (FFmpegInterface, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	binPath, err := resolveBinaryPath()
	if err != nil {
		return nil, err
	}

	instance = &ffmpeg{bin: binPath}

	return instance, nil
}

// Cleanup removes the extracted binary and its temp directory.
// Should be called when the application shuts down.
// After calling Cleanup(), New() can be called again to re-extract the binary.
func Cleanup() error {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if extractedDir == "" {
		instance = nil
		return nil
	}

	err := os.RemoveAll(extractedDir)
	if err != nil {
		return fmt.Errorf("failed to cleanup ffmpeg: %w", err)
	}

	extractedDir = ""
	instance = nil
	return nil
}
