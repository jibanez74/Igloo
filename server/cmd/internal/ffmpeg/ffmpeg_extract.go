//go:build !externalbin

package ffmpeg

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryPath extracts the embedded ffmpeg binary to a temporary
// directory and returns the path to the executable.
// embeddedBinary is defined in platform-specific files (ffmpeg_darwin_arm64.go,
// ffmpeg_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryPath() (string, error) {
	binPath, tempDir, err := mediabin.ExtractEmbedded("ffmpeg", embeddedBinary)
	if err != nil {
		return "", err
	}
	extractedDir = tempDir
	return binPath, nil
}
