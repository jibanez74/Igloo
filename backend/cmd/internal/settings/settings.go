package settings

import (
	"encoding/json"
	"fmt"
	"os"
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

		cfg := &settings{
			Debug:           os.Getenv("DEBUG") == "true",
			DownloadImages:  os.Getenv("DOWNLOAD_IMAGES") == "true",
			JellyfinToken:   os.Getenv("JELLYFIN_TOKEN"),
			TmdbKey:         os.Getenv("TMDB_API_KEY"),
			FfmpegPath:      os.Getenv("FFMPEG_PATH"),
			FfprobePath:     os.Getenv("FFPROBE_PATH"),
			StaticDir:       filepath.Join(homeDir, ".local", "share", "igloo", "static"),
			MoviesImgDir:    filepath.Join(homeDir, ".local", "share", "igloo", "static", "images", "movies"),
			StudiosImgDir:   filepath.Join(homeDir, ".local", "share", "igloo", "static", "images", "studios"),
			ArtistsImgDir:   filepath.Join(homeDir, ".local", "share", "igloo", "static", "images", "artists"),
			PostgresHost:    os.Getenv("POSTGRES_HOST"),
			PostgresPort:    os.Getenv("POSTGRES_PORT"),
			PostgresUser:    os.Getenv("POSTGRES_USER"),
			PostgresPass:    os.Getenv("POSTGRES_PASSWORD"),
			PostgresDB:      os.Getenv("POSTGRES_DB"),
			PostgresSslMode: os.Getenv("POSTGRES_SSL_MODE"),
		}

		maxCon, err := strconv.Atoi(os.Getenv("POSTGRES_MAX_CONNS"))
		if err != nil {
			maxCon = 10
		}
		cfg.PostgresMaxConns = maxCon

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

	fmt.Printf("Config: %+v\n", cfg)
	return &cfg, nil
}
