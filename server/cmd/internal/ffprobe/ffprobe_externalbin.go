//go:build externalbin

package ffprobe

import (
	"igloo/cmd/internal/mediabin"
)

// resolveBinaryPath returns an externally installed ffprobe binary. This is
// intended for tests and local development when embedding a release payload is
// unnecessary.
func resolveBinaryPath() (string, error) {
	return mediabin.ResolveExternal("ffprobe", "IGLOO_FFPROBE_PATH")
}
