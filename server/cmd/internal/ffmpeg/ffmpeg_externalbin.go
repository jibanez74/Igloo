//go:build externalbin

package ffmpeg

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryCandidate returns an externally installed ffmpeg binary. This is
// intended for tests and local development when embedding a release payload is
// unnecessary.
func resolveBinaryCandidate() (binaryCandidate, error) {
	path, err := mediabin.ResolveExternal("ffmpeg", "IGLOO_FFMPEG_PATH")
	if err != nil {
		return binaryCandidate{}, err
	}
	return binaryCandidate{path: path}, nil
}
