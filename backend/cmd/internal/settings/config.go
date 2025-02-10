package settings

import (
	"os"
	"path/filepath"
)

func getConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome != "" {
		return filepath.Join(configHome, "igloo", "config.json")
	}

	return filepath.Join(os.Getenv("HOME"), ".config", "igloo", "config.json")
}
