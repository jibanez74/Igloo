package settings

import (
	"encoding/json"
	"fmt"
	"os"
)

func createDirectories(cfg *settings) error {
	dirs := []string{
		cfg.StaticDir,
		cfg.MoviesImgDir,
		cfg.StudiosImgDir,
		cfg.ArtistsImgDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func saveConfig(cfg *settings, configPath string) error {
	file, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, file, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
