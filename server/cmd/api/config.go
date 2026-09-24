package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"igloo/cmd/internal/helpers"
)

// Defaults for the bootstrap administrator created when no admin exists.
const (
	defaultAdminName  = "Admin"
	defaultAdminEmail = "admin@sample.com"
)

// Defaults for the paths and port used when the environment says nothing.
const (
	defaultAppPort      = 8080
	defaultDBPath       = "db/igloo.db"
	defaultStaticDir    = "static"
	defaultLogsDir      = "logs"
	defaultTranscodeDir = "transcode"
)

// Every environment variable this package reads. Settings-owned values here are
// first-run seeds only — once the settings row exists the database wins and the
// value is edited from the Settings UI; the rest stay startup-driven.
//
// Two more are read outside it: the externalbin builds of the ffmpeg and ffprobe
// packages each pass IGLOO_FFMPEG_PATH / IGLOO_FFPROBE_PATH to
// mediabin.ResolveExternal, beside the binary name they override.
const (
	envDBPath                     = "DB_PATH"
	envStaticDir                  = "STATIC_DIR"
	envLogsDir                    = "LOGS_DIR"
	envTranscodeDir               = "TRANSCODE_DIR"
	envPort                       = "PORT"
	envDebug                      = "DEBUG"
	envLogToStdout                = "LOG_TO_STDOUT"
	envSessionCookieSecure        = "SESSION_COOKIE_SECURE"
	envViteDevServer              = "VITE_DEV_SERVER"
	envHLSMaxCPUTranscodes        = "HLS_MAX_CPU_TRANSCODES"
	envHLSMaxHWTranscodes         = "HLS_MAX_HW_TRANSCODES"
	envHLSMaxSessionsPerUser      = "HLS_MAX_SESSIONS_PER_USER"
	envDefaultAdminName           = "DEFAULT_ADMIN_NAME"
	envDefaultAdminEmail          = "DEFAULT_ADMIN_EMAIL"
	envDefaultAdminPassword       = "DEFAULT_ADMIN_PASSWORD"
	envTmdbAPIKey                 = "TMDB_API_KEY"
	envJellyfinAPIKey             = "JELLYFIN_API_KEY"
	envSpotifyClientID            = "SPOTIFY_CLIENT_ID"
	envSpotifyClientSecret        = "SPOTIFY_CLIENT_SECRET"
	envHardwareAccelerationDevice = "HARDWARE_ACCELERATION_DEVICE"
	envEnableWatcher              = "ENABLE_WATCHER"
	envDownloadImages             = "DOWNLOAD_IMAGES"
	envMoviesDir                  = "MOVIES_DIR"
	envShowsDir                   = "SHOWS_DIR"
	envMusicDir                   = "MUSIC_DIR"
)

type RuntimeConfig struct {
	DBPath                     string
	StaticDir                  string
	LogsDir                    string
	TranscodeDir               string
	Port                       int
	Debug                      bool
	LogToStdout                bool
	SessionCookieSecure        bool
	DefaultAdminName           string
	DefaultAdminEmail          string
	DefaultAdminPassword       string
	TmdbAPIKey                 string
	JellyfinAPIKey             string
	SpotifyClientID            string
	SpotifyClientSecret        string
	HardwareAccelerationDevice string
	EnableWatcher              bool
	DownloadImages             bool
	MoviesDir                  string
	ShowsDir                   string
	MusicDir                   string
}

func LoadRuntimeEnvFile() (string, bool, error) {
	err := helpers.LoadEnvFile(helpers.ENV_FILE)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("failed to load %s: %w", helpers.ENV_FILE, err)
	}

	return helpers.ENV_FILE, true, nil
}

// viteDevServerURL returns the configured Vite dev server origin, trimmed and
// without a trailing slash, or "" when the process is not in dev mode. Both
// readers — the SPA fallback in static_handler.go and the watch-room origin
// check in watch_room_ws.go — must normalize identically or one will accept a
// value the other mangles.
func viteDevServerURL() string {
	return strings.TrimSuffix(strings.TrimSpace(os.Getenv(envViteDevServer)), "/")
}

func NewRuntimeConfig() (RuntimeConfig, error) {
	debug := envBool(envDebug, false)

	port, err := configuredPort()
	if err != nil {
		return RuntimeConfig{}, err
	}

	config := RuntimeConfig{
		Port:                       port,
		Debug:                      debug,
		LogToStdout:                envBool(envLogToStdout, debug),
		SessionCookieSecure:        envBool(envSessionCookieSecure, false),
		DefaultAdminName:           envString(envDefaultAdminName, defaultAdminName),
		DefaultAdminEmail:          envString(envDefaultAdminEmail, defaultAdminEmail),
		DefaultAdminPassword:       strings.TrimSpace(os.Getenv(envDefaultAdminPassword)),
		TmdbAPIKey:                 strings.TrimSpace(os.Getenv(envTmdbAPIKey)),
		JellyfinAPIKey:             strings.TrimSpace(os.Getenv(envJellyfinAPIKey)),
		SpotifyClientID:            strings.TrimSpace(os.Getenv(envSpotifyClientID)),
		SpotifyClientSecret:        strings.TrimSpace(os.Getenv(envSpotifyClientSecret)),
		HardwareAccelerationDevice: envString(envHardwareAccelerationDevice, helpers.HARDWARE_ACCELERATION_DEVICE_CPU),
		EnableWatcher:              envBool(envEnableWatcher, false),
		DownloadImages:             envBool(envDownloadImages, false),
		MoviesDir:                  strings.TrimSpace(os.Getenv(envMoviesDir)),
		ShowsDir:                   strings.TrimSpace(os.Getenv(envShowsDir)),
		MusicDir:                   strings.TrimSpace(os.Getenv(envMusicDir)),
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
	return envString(envDBPath, defaultDBPath)
}

func configuredStaticDir() string {
	return envString(envStaticDir, defaultStaticDir)
}

func configuredLogsDir() string {
	return envString(envLogsDir, defaultLogsDir)
}

func configuredTranscodeDir() string {
	return envString(envTranscodeDir, defaultTranscodeDir)
}

func configuredPort() (int, error) {
	value := strings.TrimSpace(os.Getenv(envPort))
	if value == "" {
		return defaultAppPort, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", envPort, value, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid %s value %q: must be between 1 and 65535", envPort, value)
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
