//go:build externalbin

package ffmpeg

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryPath returns an externally installed ffmpeg binary. This is
// intended for tests and local development when embedding a release payload is
// unnecessary.
func resolveBinaryPath() (string, error) {
	return mediabin.ResolveExternal("ffmpeg", "IGLOO_FFMPEG_PATH")
}
