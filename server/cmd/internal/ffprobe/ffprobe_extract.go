//go:build !externalbin

package ffprobe

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryCandidate materializes the embedded zstd-compressed ffprobe
// binary on disk and returns its candidate path and extraction directory
// (empty when served from the per-version cache). embeddedCompressed is
// defined in platform-specific files (ffprobe_darwin_arm64.go,
// ffprobe_linux_amd64.go) and is populated at compile time via //go:embed.
func resolveBinaryCandidate() (binaryCandidate, error) {
	binPath, tempDir, err := mediabin.ExtractEmbeddedZstd("ffprobe", embeddedCompressed)
	if err != nil {
		return binaryCandidate{}, err
	}
	return binaryCandidate{path: binPath, extractedDir: tempDir}, nil
}
