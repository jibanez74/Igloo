//go:build !externalbin

package ffprobe

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryPath extracts the embedded ffprobe binary to a temporary
// directory and returns the path to the executable.
// embeddedBinary is defined in platform-specific files (ffprobe_darwin_arm64.go,
// ffprobe_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryPath() (string, error) {
	binPath, tempDir, err := mediabin.ExtractEmbedded("ffprobe", embeddedBinary)
	if err != nil {
		return "", err
	}
	extractedDir = tempDir
	return binPath, nil
}
