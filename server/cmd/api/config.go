package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"igloo/cmd/internal/helpers"

	"github.com/joho/godotenv"
)

type RuntimeConfig struct {
	DataDir             string
	DBPath              string
	StaticDir           string
	LogsDir             string
	TranscodeDir        string
	Port                int
	Debug               bool
	LogToStdout         bool
	SessionCookieSecure bool
}

func LoadRuntimeEnvFiles() ([]string, error) {
	explicit := strings.TrimSpace(os.Getenv(helpers.ENV_IGLOO_ENV_FILE))
	if explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", explicit, err)
		}
		return []string{explicit}, nil
	}

	var loaded []string
	seen := map[string]bool{}
	for _, candidate := range runtimeEnvFileCandidates() {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true

		if err := godotenv.Load(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return loaded, fmt.Errorf("failed to load %s: %w", candidate, err)
		}
		loaded = append(loaded, candidate)
	}

	return loaded, nil
}

func runtimeEnvFileCandidates() []string {
	candidates := []string{
		".env",
		filepath.Join("..", ".env"),
	}

	if exe, err := os.Executable(); err == nil && exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
	}

	return candidates
}

func NewRuntimeConfig() (RuntimeConfig, error) {
	debug := envBool("DEBUG", false)
	port, err := configuredPort()
	if err != nil {
		return RuntimeConfig{}, err
	}

	config := RuntimeConfig{
		DataDir:             configuredDataDir(),
		Port:                port,
		Debug:               debug,
		LogToStdout:         envBool(helpers.ENV_LOG_TO_STDOUT, debug),
		SessionCookieSecure: envBool(helpers.ENV_SESSION_COOKIE_SECURE, false),
	}
	config.DBPath = config.effectiveDBPath()
	config.StaticDir = config.effectiveStaticDir()
	config.LogsDir = config.effectiveLogsDir()
	config.TranscodeDir = config.effectiveTranscodeDir()

	return config, nil
}

func (config RuntimeConfig) effectiveDataDir() string {
	value := strings.TrimSpace(config.DataDir)
	if value == "" {
		return configuredDataDir()
	}
	return value
}

func (config RuntimeConfig) effectiveDBPath() string {
	value := strings.TrimSpace(config.DBPath)
	if value == "" {
		return configuredDBPath(config.effectiveDataDir())
	}
	return value
}

func (config RuntimeConfig) effectiveStaticDir() string {
	value := strings.TrimSpace(config.StaticDir)
	if value == "" {
		return configuredStaticDir(config.effectiveDataDir())
	}
	return value
}

func (config RuntimeConfig) effectiveLogsDir() string {
	value := strings.TrimSpace(config.LogsDir)
	if value == "" {
		return configuredLogsDir(config.effectiveDataDir())
	}
	return value
}

func (config RuntimeConfig) effectiveTranscodeDir() string {
	value := strings.TrimSpace(config.TranscodeDir)
	if value == "" {
		return configuredTranscodeDir(config.effectiveDataDir())
	}
	return value
}

func configuredDataDir() string {
	return envString(helpers.ENV_IGLOO_DATA_DIR, helpers.DEFAULT_DATA_DIR)
}

func configuredDBPath(dataDir string) string {
	return envString(helpers.ENV_DB_PATH, filepath.Join(dataDir, "igloo.db"))
}

func configuredStaticDir(dataDir string) string {
	return envString(helpers.ENV_STATIC_DIR, filepath.Join(dataDir, "static"))
}

func configuredLogsDir(dataDir string) string {
	return envString(helpers.ENV_LOGS_DIR, filepath.Join(dataDir, "logs"))
}

func configuredTranscodeDir(dataDir string) string {
	return envString(helpers.ENV_TRANSCODE_DIR, filepath.Join(dataDir, "transcode"))
}

func configuredPort() (int, error) {
	value := strings.TrimSpace(os.Getenv(helpers.ENV_PORT))
	if value == "" {
		return helpers.DEFAULT_APP_PORT, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", helpers.ENV_PORT, value, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid %s value %q: must be between 1 and 65535", helpers.ENV_PORT, value)
	}
	return port, nil
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		slog.Warn("invalid boolean value for env var, using fallback", "env", name, "value", value, "fallback", fallback)
		return fallback
	}

	return parsed
}
