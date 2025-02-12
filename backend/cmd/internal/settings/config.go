package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func loadExistingConfig(configPath string) (*settings, error) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg settings

	err = json.Unmarshal(file, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func createNewConfig(configPath string) (*settings, error) {
	cfg, err := parseEnvConfig()
	if err != nil {
		return nil, err
	}

	err = validateConfig(cfg)
	if err != nil {
		return nil, err
	}

	err = createDirectories(cfg)
	if err != nil {
		return nil, err
	}

	err = saveConfig(cfg, configPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseEnvConfig() (*settings, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	port := 8080

	portStr := os.Getenv("PORT")
	if portStr != "" {
		parsedPort, err := strconv.Atoi(portStr)
		if err == nil {
			port = parsedPort
		}
	}

	maxConns := 10

	maxConnsStr := os.Getenv("POSTGRES_MAX_CONNECTIONS")
	if maxConnsStr != "" {
		parsedMaxConns, err := strconv.Atoi(maxConnsStr)
		if err == nil {
			maxConns = parsedMaxConns
		}
	}

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if err != nil {
		redisPort = 6379
	}

	return &settings{
		Port:             fmt.Sprintf(":%d", port),
		Debug:            os.Getenv("DEBUG") == "true",
		DownloadImages:   os.Getenv("DOWNLOAD_IMAGES") == "true",
		JellyfinToken:    os.Getenv("JELLYFIN_TOKEN"),
		TmdbKey:          os.Getenv("TMDB_API_KEY"),
		FfmpegPath:       os.Getenv("FFMPEG_PATH"),
		FfprobePath:      os.Getenv("FFPROBE_PATH"),
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
		RedisHost:        redisHost,
		RedisPort:        redisPort,
	}, nil
}

func validateConfig(cfg *settings) error {
	required := map[string]string{
		"POSTGRES_HOST":     cfg.PostgresHost,
		"POSTGRES_PORT":     cfg.PostgresPort,
		"POSTGRES_USER":     cfg.PostgresUser,
		"POSTGRES_PASSWORD": cfg.PostgresPass,
		"POSTGRES_DB":       cfg.PostgresDB,
	}

	for name, value := range required {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}

	return nil
}

func (s *settings) DeleteConfigFile() error {
	configPath := filepath.Join(getConfigDir(), "config.json")

	err := os.Remove(configPath)
	if err != nil {
		return err
	}

	return nil
}

func getConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome != "" {
		return filepath.Join(configHome, "igloo")
	}

	return filepath.Join(os.Getenv("HOME"), ".config", "igloo")
}
