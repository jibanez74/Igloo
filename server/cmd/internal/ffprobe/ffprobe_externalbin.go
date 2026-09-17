//go:build externalbin

package ffprobe

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryCandidate returns an externally installed ffprobe binary. This is
// intended for tests and local development when embedding a release payload is
// unnecessary.
func resolveBinaryCandidate() (binaryCandidate, error) {
	path, err := mediabin.ResolveExternal("ffprobe", "IGLOO_FFPROBE_PATH")
	if err != nil {
		return binaryCandidate{}, err
	}
	return binaryCandidate{path: path}, nil
}
