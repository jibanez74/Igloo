//go:build !externalbin

package ffmpeg

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryCandidate extracts the embedded ffmpeg binary to a temporary
// directory and returns the candidate path and extraction directory.
// embeddedBinary is defined in platform-specific files (ffmpeg_darwin_arm64.go,
// ffmpeg_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryCandidate() (binaryCandidate, error) {
	binPath, tempDir, err := mediabin.ExtractEmbedded("ffmpeg", embeddedBinary)
	if err != nil {
		return binaryCandidate{}, err
	}
	return binaryCandidate{path: binPath, extractedDir: tempDir}, nil
}
