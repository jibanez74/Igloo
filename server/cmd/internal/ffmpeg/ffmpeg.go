package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FFmpeg struct {
	bin string
}

var (
	instance     *FFmpeg
	instanceMu   sync.Mutex
	extractedDir string
)

// New returns a singleton FFmpeg instance.
// The embedded binary is extracted to a temp directory on first call.
// Subsequent calls return the same instance without re-extracting.
func New() (*FFmpeg, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	binPath, err := extractBinary()
	if err != nil {
		return nil, err
	}

	instance = &FFmpeg{bin: binPath}

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

	if err := os.RemoveAll(extractedDir); err != nil {
		return fmt.Errorf("failed to cleanup ffmpeg: %w", err)
	}

	extractedDir = ""
	instance = nil
	return nil
}

// extractBinary writes the embedded ffmpeg binary to a temporary directory
// and returns the path to the executable.
//
// embeddedBinary is defined in platform-specific files (ffmpeg_darwin_arm64.go,
// ffmpeg_linux_amd64.go) and is populated at compile time via //go:embed.
func extractBinary() (string, error) {
	tempDir, err := os.MkdirTemp("", "igloo-ffmpeg-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	extractedDir = tempDir

	binPath := filepath.Join(tempDir, "ffmpeg")
	if err := os.WriteFile(binPath, embeddedBinary, 0755); err != nil {
		os.RemoveAll(tempDir)
		extractedDir = ""
		return "", fmt.Errorf("failed to write ffmpeg binary: %w", err)
	}

	return binPath, nil
}
