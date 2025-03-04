package config

import (
	"encoding/json"
	"fmt"
	"igloo/cmd/internal/database"
	"os"
	"path/filepath"
	"strconv"
)

type config struct {
	Settings       *database.CreateSettingsParams
	CreateSettings bool
}

func New() (*config, error) {
	var s database.CreateSettingsParams

	cfg := config{
		Settings:       &s,
		CreateSettings: false,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get users home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".config", "igloo", "igloo.json")
	sharePath := filepath.Join(homeDir, ".local", "share", "igloo")

	_, err = os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			_, err = os.Create(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to create config file: %w", err)
			}

			port, err := strconv.Atoi(os.Getenv("PORT"))
			if err != nil {
				port = 8080
			}
			s.Port = int32(port)

			debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
			if err != nil {
				debug = true
			}
			s.Debug = debug

			downloadImages, err := strconv.ParseBool(os.Getenv("DOWNLOAD_IMAGES"))
			if err != nil {
				downloadImages = false
			}
			s.DownloadImages = downloadImages

			err = cfg.createDirs(sharePath)
			if err != nil {
				return nil, fmt.Errorf("failed to create dirs: %w", err)
			}

			s.FfmpegPath = os.Getenv("FFMPEG_PATH")
			s.HardwareAcceleration = os.Getenv("HARDWARE_ACCELERATION")
			s.FfprobePath = os.Getenv("FFPROBE_PATH")
			s.TmdbApiKey = os.Getenv("TMDB_API_KEY")
			s.JellyfinToken = os.Getenv("JELLYFIN_TOKEN")

			cfg.CreateSettings = true

			jsonData, err := json.MarshalIndent(s, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal settings: %w", err)
			}

			err = os.WriteFile(configPath, jsonData, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to write config file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to check config file: %w", err)
		}
	}

	return &cfg, nil
}
