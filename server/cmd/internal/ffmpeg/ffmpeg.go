package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"igloo/cmd/internal/mediabin"
)

type FFmpegInterface interface {
	RunHLS(ctx context.Context, params HLSParams, onExit func(exitErr error, stderrTail []string)) (*exec.Cmd, error)
	ExtractSubtitleAsWebVTT(ctx context.Context, sourcePath string, streamIndex int64) ([]byte, error)
	Capabilities() Capabilities
}

type ffmpeg struct {
	bin          string
	capabilities Capabilities
}

var _ FFmpegInterface = (*ffmpeg)(nil)

func (f *ffmpeg) Capabilities() Capabilities {
	return f.capabilities
}

var (
	instance     *ffmpeg
	instanceMu   sync.Mutex
	extractedDir string
)

// New returns a singleton FFmpeg implementation.
// resolveBinaryPath is defined in build-tag-specific files:
// embedded mode extracts a release payload; externalbin mode uses host ffmpeg.
func New() (FFmpegInterface, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	binPath, err := resolveBinaryPath()
	if err != nil {
		return nil, err
	}

	// Confirm the binary actually executes before accepting it. The individual
	// capability probes below tolerate failures, so without this a corrupt,
	// wrong-arch, or non-executable ffmpeg would boot with empty capabilities and
	// only fail at the first transcode. The server must not boot without a working
	// ffmpeg.
	if _, err := runFFmpegProbe(binPath, "-version"); err != nil {
		return nil, fmt.Errorf("ffmpeg binary at %s is not executable: %w", binPath, err)
	}

	instance = &ffmpeg{
		bin:          binPath,
		capabilities: probeCapabilities(binPath),
	}

	return instance, nil
}

// Cleanup removes the extracted binary and its temp directory.
// Should be called when the application shuts down.
// After calling Cleanup(), New() can be called again to re-extract the binary.
func Cleanup() error {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if extractedDir == "" {
		instance = nil
		return nil
	}

	if err := mediabin.CleanupExtracted("ffmpeg", extractedDir); err != nil {
		return err
	}

	extractedDir = ""
	instance = nil
	return nil
}
