package helpers

import (
	"fmt"
	"os"
)

// GetOrCreateDir ensures path is a readable directory and reports whether it was created.
func GetOrCreateDir(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("path cannot be empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(path, 0o755)
			if err != nil {
				return false, fmt.Errorf("failed to create directory: %w", err)
			}

			return true, nil
		}

		if os.IsPermission(err) {
			return false, fmt.Errorf("permission denied: %w", err)
		}

		return false, fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		return false, fmt.Errorf("path is not a directory: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			return false, fmt.Errorf("permission denied reading directory: %w", err)
		}

		return false, fmt.Errorf("failed to open directory: %w", err)
	}
	defer f.Close()

	return false, nil
}
