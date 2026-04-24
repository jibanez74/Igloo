//go:build systembin

package ffmpeg

// resolveBinaryPath returns the path to the system-installed ffmpeg binary.
// Used in Docker where Jellyfin FFmpeg is installed via the .deb package.
func resolveBinaryPath() (string, error) {
	return "/usr/lib/jellyfin-ffmpeg/ffmpeg", nil
}
