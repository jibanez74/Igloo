package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"unicode"

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
	envTrustedProxies             = "TRUSTED_PROXIES"
	envViteDevServer              = "VITE_DEV_SERVER"
	envHLSMaxCPUTranscodes        = "HLS_MAX_CPU_TRANSCODES"
	envHLSMaxHWTranscodes         = "HLS_MAX_HW_TRANSCODES"
	envHLSMaxSessionsPerUser      = "HLS_MAX_SESSIONS_PER_USER"
	envDefaultAdminName           = "DEFAULT_ADMIN_NAME"
	envDefaultAdminEmail          = "DEFAULT_ADMIN_EMAIL"
	envDefaultAdminPassword       = "DEFAULT_ADMIN_PASSWORD"
	envTmdbAPIKey                 = "TMDB_API_KEY"
	envSpotifyClientID            = "SPOTIFY_CLIENT_ID"
	envSpotifyClientSecret        = "SPOTIFY_CLIENT_SECRET"
	envEnableAppleLookups         = "ENABLE_APPLE_LOOKUPS"
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
	TrustedProxies             []netip.Prefix
	DefaultAdminName           string
	DefaultAdminEmail          string
	DefaultAdminPassword       string
	TmdbAPIKey                 string
	SpotifyClientID            string
	SpotifyClientSecret        string
	EnableAppleLookups         bool
	HardwareAccelerationDevice string
	EnableWatcher              bool
	DownloadImages             bool
	MoviesDir                  string
	ShowsDir                   string
	MusicDir                   string
}

// envFile is the optional startup seed file, read from the working directory.
const envFile = ".env"

// LoadRuntimeEnvFile reports whether envFile was found and loaded.
func LoadRuntimeEnvFile() (bool, error) {
	err := helpers.LoadEnvFile(envFile)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to load %s: %w", envFile, err)
	}
	return true, nil
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

	trustedProxies, err := parseTrustedProxies(os.Getenv(envTrustedProxies))
	if err != nil {
		return RuntimeConfig{}, err
	}

	config := RuntimeConfig{
		DBPath:                     envString(envDBPath, defaultDBPath),
		StaticDir:                  envString(envStaticDir, defaultStaticDir),
		LogsDir:                    envString(envLogsDir, defaultLogsDir),
		TranscodeDir:               envString(envTranscodeDir, defaultTranscodeDir),
		Port:                       port,
		Debug:                      debug,
		LogToStdout:                envBool(envLogToStdout, debug),
		SessionCookieSecure:        envBool(envSessionCookieSecure, false),
		TrustedProxies:             trustedProxies,
		DefaultAdminName:           envString(envDefaultAdminName, defaultAdminName),
		DefaultAdminEmail:          envString(envDefaultAdminEmail, defaultAdminEmail),
		DefaultAdminPassword:       strings.TrimSpace(os.Getenv(envDefaultAdminPassword)),
		TmdbAPIKey:                 strings.TrimSpace(os.Getenv(envTmdbAPIKey)),
		SpotifyClientID:            strings.TrimSpace(os.Getenv(envSpotifyClientID)),
		SpotifyClientSecret:        strings.TrimSpace(os.Getenv(envSpotifyClientSecret)),
		EnableAppleLookups:         envBool(envEnableAppleLookups, true),
		HardwareAccelerationDevice: envString(envHardwareAccelerationDevice, helpers.HARDWARE_ACCELERATION_DEVICE_CPU),
		EnableWatcher:              envBool(envEnableWatcher, false),
		DownloadImages:             envBool(envDownloadImages, false),
		MoviesDir:                  strings.TrimSpace(os.Getenv(envMoviesDir)),
		ShowsDir:                   strings.TrimSpace(os.Getenv(envShowsDir)),
		MusicDir:                   strings.TrimSpace(os.Getenv(envMusicDir)),
	}
	return config, nil
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

// parseTrustedProxies reads TRUSTED_PROXIES: reverse proxy addresses, each an
// IP or a CIDR, separated by commas or spaces. A bare IP trusts that one
// address. An IPv4-mapped IPv6 address folds to IPv4, the form the client IP
// middleware compares peers in. A bad entry fails startup rather than leaving
// the proxy untrusted without a word.
func parseTrustedProxies(value string) ([]netip.Prefix, error) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	if len(fields) == 0 {
		return nil, nil
	}

	prefixes := make([]netip.Prefix, 0, len(fields))
	for _, field := range fields {
		if strings.Contains(field, "/") {
			prefix, err := netip.ParsePrefix(field)
			if err != nil {
				return nil, fmt.Errorf("invalid %s entry %q: %w", envTrustedProxies, field, err)
			}
			addr := prefix.Addr()
			bits := prefix.Bits()
			if addr.Is4In6() {
				if bits < 96 {
					return nil, fmt.Errorf("invalid %s entry %q: an IPv4-mapped prefix needs at least /96", envTrustedProxies, field)
				}
				addr, bits = addr.Unmap(), bits-96
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr, bits).Masked())
			continue
		}

		addr, err := netip.ParseAddr(field)
		if err != nil {
			return nil, fmt.Errorf("invalid %s entry %q: %w", envTrustedProxies, field, err)
		}
		if addr.Zone() != "" {
			return nil, fmt.Errorf("invalid %s entry %q: zoned addresses are not supported", envTrustedProxies, field)
		}
		addr = addr.Unmap()
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return prefixes, nil
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		slog.Warn("invalid boolean value for env var, using fallback", "env", name, "value", value, "fallback", fallback)
		return fallback
	}

	return parsed
}
