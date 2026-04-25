//go:build systembin

package ffprobe

// resolveBinaryPath returns the path to the system-installed ffprobe binary.
// Used in Docker where Jellyfin FFmpeg is installed via the .deb package.
func resolveBinaryPath() (string, error) {
	return "/usr/lib/jellyfin-ffmpeg/ffprobe", nil
}
