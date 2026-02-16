package ffprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FfprobeInterface interface {
	GetMetadata(filePath string) (*FfprobeResult, error)
}

type ffprobe struct {
	bin string // Path to the extracted ffprobe binary
}

// Compile-time check to ensure ffprobe implements FfprobeInterface.
// This will cause a compilation error if the interface is not satisfied.
var _ FfprobeInterface = (*ffprobe)(nil)

// Package-level state for the singleton pattern.
// These variables are protected by instanceMu to ensure thread safety.
var (
	instance     *ffprobe   // The singleton ffprobe instance
	instanceMu   sync.Mutex // Protects instance and extractedDir
	extractedDir string     // Path to the temp directory containing the binary
)

// New returns a singleton ffprobe instance.
// The embedded binary is extracted to a temp directory on first call.
// Subsequent calls return the same instance without re-extracting.
//
// Thread-safe: multiple goroutines can call New() concurrently.
//
// Returns an error if:
//   - The temp directory cannot be created
//   - The binary cannot be written to disk
func New() (FfprobeInterface, error) {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	// Return existing instance if already initialized
	if instance != nil {
		return instance, nil
	}

	// Extract the embedded binary to a temp directory
	binPath, err := extractBinary()
	if err != nil {
		return nil, err
	}

	instance = &ffprobe{bin: binPath}

	return instance, nil
}

// Cleanup removes the extracted binary and its temp directory.
// Should be called when the application shuts down to clean up resources.
//
// After calling Cleanup(), New() can be called again to re-extract the binary.
// This is useful for long-running applications that need to release resources.
//
// Thread-safe: can be called concurrently with New().
func Cleanup() error {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	// Nothing to clean up if not initialized
	if extractedDir == "" {
		return nil
	}

	// Remove the entire temp directory (includes the binary)
	err := os.RemoveAll(extractedDir)
	if err != nil {
		return fmt.Errorf("failed to cleanup ffprobe: %w", err)
	}

	// Reset state so New() can re-extract if called again
	extractedDir = ""
	instance = nil

	return nil
}

// extractBinary writes the embedded ffprobe binary to a temporary directory
// and returns the path to the executable.
//
// The binary is written with 0755 permissions (rwxr-xr-x) to make it executable.
//
// Note: embeddedBinary is defined in platform-specific files (e.g., ffprobe_darwin_arm64.go)
// and is populated at compile time via //go:embed directives.
func extractBinary() (string, error) {
	// Create a unique temp directory for this application instance
	// The pattern "igloo-ffprobe-*" helps identify these directories if cleanup fails
	tempDir, err := os.MkdirTemp("", "igloo-ffprobe-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Store the temp directory path for cleanup
	extractedDir = tempDir

	// Write the embedded binary to the temp directory
	binPath := filepath.Join(tempDir, "ffprobe")
	err = os.WriteFile(binPath, embeddedBinary, 0755)
	if err != nil {
		// Clean up on failure to avoid orphaned directories
		os.RemoveAll(tempDir)
		extractedDir = ""
		return "", fmt.Errorf("failed to write ffprobe binary: %w", err)
	}

	return binPath, nil
}
