//go:build !systembin

package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveBinaryPath extracts the embedded ffmpeg binary to a temporary
// directory and returns the path to the executable.
// embeddedBinary is defined in platform-specific files (ffmpeg_darwin_arm64.go,
// ffmpeg_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryPath() (string, error) {
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
