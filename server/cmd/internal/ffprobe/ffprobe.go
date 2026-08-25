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
	GetMetadata(ctx context.Context, filePath string) (*FfprobeResult, error)
	GetAudioMetadata(ctx context.Context, filePath string) (*FfprobeResult, error)
	KeyframeAtOrBefore(ctx context.Context, filePath string, streamIndex int64, targetSec float64) (float64, error)
}

type ffprobe struct {
	bin string
}

type binaryCandidate struct {
	path         string
	extractedDir string
}

var _ FfprobeInterface = (*ffprobe)(nil)

var (
	instance     *ffprobe
	instanceMu   sync.Mutex
	extractedDir string
)

// New returns a singleton ffprobe instance.
// resolveBinaryCandidate is defined in build-tag-specific files:
// embedded mode extracts a release payload; externalbin mode uses host ffprobe.
func New() (FfprobeInterface, error) {
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

func initializeCandidate(candidate binaryCandidate) (*ffprobe, error) {
	// Confirm the binary actually executes; path resolution alone does not catch a
	// corrupt, wrong-arch, or non-executable ffprobe. The server must not boot
	// without a working ffprobe.
	err := verifyExecutable(candidate.path)
	if err != nil {
		cleanupErr := mediabin.CleanupExtracted("ffprobe", candidate.extractedDir)
		if cleanupErr != nil {
			return nil, fmt.Errorf(
				"ffprobe binary at %s is not executable: %w (cleanup failed: %v)",
				candidate.path,
				err,
				cleanupErr,
			)
		}
		return nil, fmt.Errorf("ffprobe binary at %s is not executable: %w", candidate.path, err)
	}

	return &ffprobe{bin: candidate.path}, nil
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
// After calling Cleanup(), New() can be called again to re-extract the binary.
func Cleanup() error {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if extractedDir == "" {
		instance = nil
		return nil
	}

	err := mediabin.CleanupExtracted("ffprobe", extractedDir)
	if err != nil {
		return err
	}

	extractedDir = ""
	instance = nil

	return nil
}
