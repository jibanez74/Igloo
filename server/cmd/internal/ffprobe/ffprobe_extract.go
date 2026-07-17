//go:build !externalbin

package ffprobe

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryCandidate extracts the embedded ffprobe binary to a temporary
// directory and returns its candidate path and extraction directory.
// embeddedBinary is defined in platform-specific files (ffprobe_darwin_arm64.go,
// ffprobe_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryCandidate() (binaryCandidate, error) {
	binPath, tempDir, err := mediabin.ExtractEmbedded("ffprobe", embeddedBinary)
	if err != nil {
		return binaryCandidate{}, err
	}
	return binaryCandidate{path: binPath, extractedDir: tempDir}, nil
}
