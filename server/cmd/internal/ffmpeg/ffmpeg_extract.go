//go:build !externalbin

package ffmpeg

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryCandidate materializes the embedded zstd-compressed ffmpeg
// binary on disk and returns the candidate path and extraction directory
// (empty when served from the per-version cache). embeddedCompressed is
// defined in platform-specific files (ffmpeg_darwin_arm64.go,
// ffmpeg_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryCandidate() (binaryCandidate, error) {
	binPath, tempDir, err := mediabin.ExtractEmbeddedZstd("ffmpeg", embeddedCompressed)
	if err != nil {
		return binaryCandidate{}, err
	}
	return binaryCandidate{path: binPath, extractedDir: tempDir}, nil
}
