//go:build !systembin

package ffprobe

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveBinaryPath extracts the embedded ffprobe binary to a temporary
// directory and returns the path to the executable.
// embeddedBinary is defined in platform-specific files (ffprobe_darwin_arm64.go,
// ffprobe_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryPath() (string, error) {
	if len(embeddedBinary) == 0 {
		return "", fmt.Errorf("ffprobe binary is missing: embedded payload is empty (binary was not included at compile time)")
	}

	tempDir, err := os.MkdirTemp("", "igloo-ffprobe-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	extractedDir = tempDir

	binPath := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(binPath, embeddedBinary, 0755)
	if err != nil {
		os.RemoveAll(tempDir)
		extractedDir = ""
		return "", fmt.Errorf("failed to write ffprobe binary: %w", err)
	}

	return binPath, nil
}
