package helpers

import (
	"fmt"
	"os"
)

func CreateDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(path, 0755)
			if err != nil {
				return fmt.Errorf("failed to create dir: %w", err)
			}

			return nil
		}

		return fmt.Errorf("failed to check dir: %w", err)
	}

	return nil
}
