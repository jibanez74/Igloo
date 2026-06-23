package ffprobe

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/mediabin"
)

const versionCheckTimeout = 5 * time.Second

type FfprobeInterface interface {
	GetMetadata(filePath string) (*FfprobeResult, error)
	GetAudioMetadata(filePath string) (*FfprobeResult, error)
}

type ffprobe struct {
	bin string
}

var _ FfprobeInterface = (*ffprobe)(nil)

var (
	instance     *ffprobe
	instanceMu   sync.Mutex
	extractedDir string
)

// New returns a singleton ffprobe instance.
// resolveBinaryPath is defined in build-tag-specific files:
// embedded mode extracts a release payload; externalbin mode uses host ffprobe.
func New() (FfprobeInterface, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance != nil {
		return instance, nil
	}

	binPath, err := resolveBinaryPath()
	if err != nil {
		return nil, err
	}

	// Confirm the binary actually executes; path resolution alone does not catch a
	// corrupt, wrong-arch, or non-executable ffprobe. The server must not boot
	// without a working ffprobe.
	if err := verifyExecutable(binPath); err != nil {
		return nil, fmt.Errorf("ffprobe binary at %s is not executable: %w", binPath, err)
	}

	instance = &ffprobe{bin: binPath}

	return instance, nil
}

func verifyExecutable(binPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, binPath, "-version").CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// Cleanup removes the extracted binary and its temp directory.
// Should be called when the application shuts down.
func Cleanup() error {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if extractedDir == "" {
		instance = nil
		return nil
	}

	if err := mediabin.CleanupExtracted("ffprobe", extractedDir); err != nil {
		return err
	}

	extractedDir = ""
	instance = nil

	return nil
}
