package main

import (
  "errors"
  "fmt"
  "log/slog"
  "os"
  "strconv"
  "strings"

  "igloo/cmd/internal/helpers"

  "github.com/joho/godotenv"
)

type RuntimeConfig struct {
  DBPath              string
  StaticDir           string
  LogsDir             string
  TranscodeDir        string
  Port                int
  Debug               bool
  LogToStdout         bool
  SessionCookieSecure bool
}

func LoadRuntimeEnvFile() (string, bool, error) {
  err := godotenv.Load(helpers.ENV_FILE)
  if err != nil {
    if errors.Is(err, os.ErrNotExist) {
      return "", false, nil
    }

    return "", false, fmt.Errorf("failed to load %s: %w", helpers.ENV_FILE, err)
  }

  return helpers.ENV_FILE, true, nil
}

func NewRuntimeConfig() (RuntimeConfig, error) {
  debug := envBool("DEBUG", false)

  port, err := configuredPort()
  if err != nil {
    return RuntimeConfig{}, err
  }

  config := RuntimeConfig{
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

func (config RuntimeConfig) effectiveDBPath() string {
  value := strings.TrimSpace(config.DBPath)
  if value == "" {
    return configuredDBPath()
  }
  return value
}

func (config RuntimeConfig) effectiveDataDir() string {
  return ""
}

func (config RuntimeConfig) effectiveStaticDir() string {
  value := strings.TrimSpace(config.StaticDir)
  if value == "" {
    return configuredStaticDir()
  }
  return value
}

func (config RuntimeConfig) effectiveLogsDir() string {
  value := strings.TrimSpace(config.LogsDir)
  if value == "" {
    return configuredLogsDir()
  }
  return value
}

func (config RuntimeConfig) effectiveTranscodeDir() string {
  value := strings.TrimSpace(config.TranscodeDir)
  if value == "" {
    return configuredTranscodeDir()
  }
  return value
}

func configuredDBPath() string {
  return envString(helpers.ENV_DB_PATH, helpers.DEFAULT_DB_PATH)
}

func configuredStaticDir(_ ...string) string {
  return envString(helpers.ENV_STATIC_DIR, helpers.DEFAULT_STATIC_DIR)
}

func configuredLogsDir(_ ...string) string {
  return envString(helpers.ENV_LOGS_DIR, helpers.DEFAULT_LOGS_DIR)
}

func configuredTranscodeDir(_ ...string) string {
  return envString(helpers.ENV_TRANSCODE_DIR, helpers.DEFAULT_TRANSCODE_DIR)
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
