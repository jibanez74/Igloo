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

type binaryCandidate struct {
	path         string
	extractedDir string
}

var _ FFmpegInterface = (*ffmpeg)(nil)

func (f *ffmpeg) Capabilities() Capabilities {
	return cloneCapabilities(f.capabilities)
}

var (
	instance     *ffmpeg
	instanceMu   sync.Mutex
	extractedDir string
)

// New returns a singleton FFmpeg implementation.
// resolveBinaryCandidate is defined in build-tag-specific files:
// embedded mode extracts a release payload; externalbin mode uses host ffmpeg.
func New() (FFmpegInterface, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	candidate, err := resolveBinaryCandidate()
	if err != nil {
		return nil, err
	}

	created, err := initializeCandidate(candidate)
	if err != nil {
		return nil, err
	}

	instance = created
	extractedDir = candidate.extractedDir

	return instance, nil
}

func initializeCandidate(candidate binaryCandidate) (*ffmpeg, error) {
	// Confirm the binary actually executes before accepting it. The individual
	// capability probes below tolerate failures, so without this a corrupt,
	// wrong-arch, or non-executable ffmpeg would boot with empty capabilities and
	// only fail at the first transcode. The server must not boot without a working
	// ffmpeg.
	versionOutput, err := runFFmpegProbe(candidate.path, "-version")
	if err != nil {
		cleanupErr := mediabin.CleanupExtracted("ffmpeg", candidate.extractedDir)
		if cleanupErr != nil {
			return nil, fmt.Errorf(
				"ffmpeg binary at %s is not executable: %w (cleanup failed: %v)",
				candidate.path,
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf("ffmpeg binary at %s is not executable: %w", candidate.path, err)
	}

	return &ffmpeg{
		bin:          candidate.path,
		capabilities: probeCapabilities(candidate.path, versionOutput),
	}, nil
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

	err := mediabin.CleanupExtracted("ffmpeg", extractedDir)
	if err != nil {
		return err
	}

	extractedDir = ""
	instance = nil
	return nil
}
