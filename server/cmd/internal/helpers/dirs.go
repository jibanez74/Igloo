package helpers

import (
	"fmt"
	"os"
)

// dirMode is the permission mode for every directory Igloo creates for itself:
// owner-writable, world-readable. Media libraries are not created here, and the
// extracted FFmpeg binaries in cmd/internal/mediabin deliberately use a
// stricter mode of their own.
const dirMode = 0o755

// ValidateDir reports whether path is an existing directory Igloo can read.
// Permission failures are distinguished from every other stat or open failure,
// because they are the ones an operator can fix.
func ValidateDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: %w", err)
		}

		return fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied reading directory: %w", err)
		}

		return fmt.Errorf("failed to open directory: %w", err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close directory: %w", err)
	}

	return nil
}

// GetOrCreateDir ensures path is a readable directory and reports whether it was created.
func GetOrCreateDir(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("path cannot be empty")
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.MkdirAll(path, dirMode)
		if err != nil {
			return false, fmt.Errorf("failed to create directory: %w", err)
		}

		return true, nil
	}

	return false, ValidateDir(path)
}
