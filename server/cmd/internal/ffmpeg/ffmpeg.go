package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
// The embedded binary is extracted to a temp directory on first call.
// Subsequent calls return the same instance without re-extracting.
func New() (FFmpegInterface, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	binPath, err := extractBinary()
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

// extractBinary writes the embedded ffmpeg binary to a temporary directory
// and returns the path to the executable.
// embeddedBinary is defined in platform-specific files (ffmpeg_darwin_arm64.go,
// ffmpeg_linux_amd64.go) and is populated at compile time via //go:embed.
func extractBinary() (string, error) {
	tempDir, err := os.MkdirTemp("", "igloo-ffmpeg-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	extractedDir = tempDir

	binPath := filepath.Join(tempDir, "ffmpeg")
	writeErr := os.WriteFile(binPath, embeddedBinary, 0755)
	if writeErr != nil {
		os.RemoveAll(tempDir)
		extractedDir = ""
		return "", fmt.Errorf("failed to write ffmpeg binary: %w", writeErr)
	}

	return binPath, nil
}
