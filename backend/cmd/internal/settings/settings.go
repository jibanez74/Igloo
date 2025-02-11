package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type settings struct {
	Debug            bool   `json:"debug"`
	DownloadImages   bool   `json:"download_images"`
	TmdbKey          string `json:"tmdb_key"`
	JellyfinToken    string `json:"jellyfin_token"`
	FfmpegPath       string `json:"ffmpeg_path"`
	FfprobePath      string `json:"ffprobe_path"`
	StaticDir        string `json:"static_dir"`
	MoviesImgDir     string `json:"movies_img_dir"`
	StudiosImgDir    string `json:"studios_img_dir"`
	ArtistsImgDir    string `json:"artists_img_dir"`
	PostgresHost     string `json:"postgres_host"`
	PostgresPort     string `json:"postgres_port"`
	PostgresUser     string `json:"postgres_user"`
	PostgresPass     string `json:"postgres_pass"`
	PostgresDB       string `json:"postgres_db"`
	PostgresSslMode  string `json:"postgres_ssl_mode"`
	PostgresMaxConns int    `json:"postgres_max_conns"`
}

func New() (*settings, error) {
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)

	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}

		// Get and validate ffmpeg path
		ffmpegPath := os.Getenv("FFMPEG_PATH")
		if ffmpegPath == "" {
			// Try to find ffmpeg in PATH as fallback
			ffmpegPath, err = exec.LookPath("ffmpeg")
			if err != nil {
				return nil, fmt.Errorf("FFMPEG_PATH not set and ffmpeg not found in PATH: %w", err)
			}
		}

		// Get and validate ffprobe path
		ffprobePath := os.Getenv("FFPROBE_PATH")
		if ffprobePath == "" {
			// Try to find ffprobe in PATH as fallback
			ffprobePath, err = exec.LookPath("ffprobe")
			if err != nil {
				return nil, fmt.Errorf("FFPROBE_PATH not set and ffprobe not found in PATH: %w", err)
			}
		}

		// Get and validate database settings
		maxConns := 50 // Default value
		if maxConnsStr := os.Getenv("POSTGRES_MAX_CONNECTIONS"); maxConnsStr != "" {
			maxConns, err = strconv.Atoi(maxConnsStr)
			if err != nil {
				return nil, fmt.Errorf("invalid POSTGRES_MAX_CONNECTIONS value: %w", err)
			}
		}

		cfg := &settings{
			Debug:            os.Getenv("DEBUG") == "true",
			DownloadImages:   os.Getenv("DOWNLOAD_IMAGES") == "true",
			JellyfinToken:    os.Getenv("JELLYFIN_TOKEN"),
			TmdbKey:          os.Getenv("TMDB_API_KEY"),
			FfmpegPath:       ffmpegPath,
			FfprobePath:      ffprobePath,
			StaticDir:        filepath.Join(homeDir, ".local", "share", "igloo", "static"),
			MoviesImgDir:     filepath.Join(homeDir, ".local", "share", "igloo", "static", "images", "movies"),
			StudiosImgDir:    filepath.Join(homeDir, ".local", "share", "igloo", "static", "images", "studios"),
			ArtistsImgDir:    filepath.Join(homeDir, ".local", "share", "igloo", "static", "images", "artists"),
			PostgresHost:     os.Getenv("POSTGRES_HOST"),
			PostgresPort:     os.Getenv("POSTGRES_PORT"),
			PostgresUser:     os.Getenv("POSTGRES_USER"),
			PostgresPass:     os.Getenv("POSTGRES_PASSWORD"),
			PostgresDB:       os.Getenv("POSTGRES_DB"),
			PostgresSslMode:  os.Getenv("POSTGRES_SSL_MODE"),
			PostgresMaxConns: maxConns,
		}

		// Validate required database settings
		if cfg.PostgresHost == "" {
			return nil, errors.New("POSTGRES_HOST is required")
		}
		if cfg.PostgresPort == "" {
			return nil, errors.New("POSTGRES_PORT is required")
		}
		if cfg.PostgresUser == "" {
			return nil, errors.New("POSTGRES_USER is required")
		}
		if cfg.PostgresPass == "" {
			return nil, errors.New("POSTGRES_PASSWORD is required")
		}
		if cfg.PostgresDB == "" {
			return nil, errors.New("POSTGRES_DB is required")
		}

		dirs := [4]string{cfg.StaticDir, cfg.MoviesImgDir, cfg.StudiosImgDir, cfg.ArtistsImgDir}
		for _, dir := range dirs {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		file, err := json.MarshalIndent(cfg, "", "    ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config: %w", err)
		}

		err = os.WriteFile(configPath, file, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to write config: %w", err)
		}

		return cfg, nil
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg settings

	err = json.Unmarshal(file, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate ffmpeg and ffprobe paths when loading from config
	if cfg.FfmpegPath == "" {
		return nil, errors.New("ffmpeg path is not set in config")
	}
	if cfg.FfprobePath == "" {
		return nil, errors.New("ffprobe path is not set in config")
	}

	// Verify the paths still exist and are executable
	if _, err := exec.LookPath(cfg.FfmpegPath); err != nil {
		return nil, fmt.Errorf("ffmpeg not found or not executable at %s: %w", cfg.FfmpegPath, err)
	}
	if _, err := exec.LookPath(cfg.FfprobePath); err != nil {
		return nil, fmt.Errorf("ffprobe not found or not executable at %s: %w", cfg.FfprobePath, err)
	}

	fmt.Printf("Config loaded: %+v\n", cfg)
	return &cfg, nil
}
