package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func clearRuntimeConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		envDBPath,
		envStaticDir,
		envLogsDir,
		envTranscodeDir,
		envPort,
		envLogToStdout,
		envSessionCookieSecure,
		"DEBUG",
		envDefaultAdminName,
		envDefaultAdminEmail,
		envDefaultAdminPassword,
		envTmdbAPIKey,
		envSpotifyClientID,
		envSpotifyClientSecret,
		envEnableAppleLookups,
		envHardwareAccelerationDevice,
		envEnableWatcher,
		envDownloadImages,
		envTrustedProxies,
		envMoviesDir,
		envShowsDir,
		envMusicDir,
	} {
		t.Setenv(key, "")
	}
}

func TestNewRuntimeConfig_DefaultPaths(t *testing.T) {
	clearRuntimeConfigEnv(t)

	cfg, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}

	if cfg.DBPath != defaultDBPath {
		t.Fatalf("expected derived DB path, got %q", cfg.DBPath)
	}
	if cfg.StaticDir != defaultStaticDir {
		t.Fatalf("expected derived static dir, got %q", cfg.StaticDir)
	}
	if cfg.LogsDir != defaultLogsDir {
		t.Fatalf("expected derived logs dir, got %q", cfg.LogsDir)
	}
	if cfg.TranscodeDir != defaultTranscodeDir {
		t.Fatalf("expected derived transcode dir, got %q", cfg.TranscodeDir)
	}
}

func TestNewRuntimeConfig_ExplicitPathsOverrideDefaults(t *testing.T) {
	clearRuntimeConfigEnv(t)

	dbPath := filepath.Join(t.TempDir(), "custom.db")
	staticDir := filepath.Join(t.TempDir(), "static")
	logsDir := filepath.Join(t.TempDir(), "logs")
	transcodeDir := filepath.Join(t.TempDir(), "transcode")

	t.Setenv(envDBPath, dbPath)
	t.Setenv(envStaticDir, staticDir)
	t.Setenv(envLogsDir, logsDir)
	t.Setenv(envTranscodeDir, transcodeDir)

	cfg, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}

	if cfg.DBPath != dbPath {
		t.Fatalf("expected DB path override %q, got %q", dbPath, cfg.DBPath)
	}
	if cfg.StaticDir != staticDir {
		t.Fatalf("expected static dir override %q, got %q", staticDir, cfg.StaticDir)
	}
	if cfg.LogsDir != logsDir {
		t.Fatalf("expected logs dir override %q, got %q", logsDir, cfg.LogsDir)
	}
	if cfg.TranscodeDir != transcodeDir {
		t.Fatalf("expected transcode dir override %q, got %q", transcodeDir, cfg.TranscodeDir)
	}
}

func TestNewRuntimeConfig_PortHonoredWithoutDebug(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv("DEBUG", "false")
	t.Setenv(envPort, "4242")

	cfg, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig failed: %v", err)
	}

	if cfg.Port != 4242 {
		t.Fatalf("expected PORT to be honored without DEBUG, got %d", cfg.Port)
	}
}

func TestNewRuntimeConfig_RejectsInvalidPort(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv(envPort, "not-a-port")

	_, err := NewRuntimeConfig()
	if err == nil {
		t.Fatal("expected invalid PORT to return an error")
	}
}

func TestLoadRuntimeEnvFile_LoadsWorkingDirectoryEnvFile(t *testing.T) {
	envDir := t.TempDir()
	changeWorkingDirectory(t, envDir)

	if err := os.WriteFile(".env", []byte("IGLOO_TEST_ENV_FILE_VALUE=loaded\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	const key = "IGLOO_TEST_ENV_FILE_VALUE"
	// t.Setenv restores the key at cleanup; the unset leaves it absent so
	// the file's value is the one that lands.
	t.Setenv(key, "")
	os.Unsetenv(key)

	loaded, err := LoadRuntimeEnvFile()
	if err != nil {
		t.Fatalf("LoadRuntimeEnvFile failed: %v", err)
	}

	if !loaded {
		t.Fatal("expected .env to be loaded")
	}
	if got := os.Getenv(key); got != "loaded" {
		t.Fatalf("expected env file value to load, got %q", got)
	}
}

func TestLoadRuntimeEnvFile_MissingEnvFileIsIgnored(t *testing.T) {
	changeWorkingDirectory(t, t.TempDir())

	loaded, err := LoadRuntimeEnvFile()
	if err != nil {
		t.Fatalf("LoadRuntimeEnvFile failed: %v", err)
	}
	if loaded {
		t.Fatal("expected missing .env to be ignored")
	}
}

func TestNewRuntimeConfig_BooleansAndDefaultAdmin(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv(envDebug, "true")
	t.Setenv(envSessionCookieSecure, "true")
	t.Setenv(envDefaultAdminName, "Ops")
	t.Setenv(envDefaultAdminEmail, "ops@example.com")
	t.Setenv(envDefaultAdminPassword, " top secret ")

	config, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig: %v", err)
	}
	if !config.Debug || !config.SessionCookieSecure {
		t.Fatalf("Debug = %t, SessionCookieSecure = %t, want both true", config.Debug, config.SessionCookieSecure)
	}
	// LOG_TO_STDOUT follows DEBUG when it is unset.
	if !config.LogToStdout {
		t.Fatal("LogToStdout = false, want it to follow DEBUG=true")
	}
	if config.DefaultAdminName != "Ops" || config.DefaultAdminEmail != "ops@example.com" || config.DefaultAdminPassword != "top secret" {
		t.Fatalf("default admin = %q %q %q, want the trimmed env values", config.DefaultAdminName, config.DefaultAdminEmail, config.DefaultAdminPassword)
	}

	t.Setenv(envLogToStdout, "false")
	t.Setenv(envSessionCookieSecure, "not a bool")
	t.Setenv(envDefaultAdminName, "")
	t.Setenv(envDefaultAdminEmail, "")
	config, err = NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig: %v", err)
	}
	if config.LogToStdout {
		t.Fatal("LogToStdout = true, want the explicit false to win over DEBUG")
	}
	if config.SessionCookieSecure {
		t.Fatal("SessionCookieSecure = true, want an unparsable value to fall back to false")
	}
	if config.DefaultAdminName != defaultAdminName || config.DefaultAdminEmail != defaultAdminEmail {
		t.Fatalf("default admin = %q %q, want the built-in defaults", config.DefaultAdminName, config.DefaultAdminEmail)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		value string
		want  []string
	}{
		{value: "", want: nil},
		{value: " , ", want: nil},
		{value: "127.0.0.1", want: []string{"127.0.0.1/32"}},
		{value: "127.0.0.1, ::1", want: []string{"127.0.0.1/32", "::1/128"}},
		{value: "10.0.0.7/24 172.16.0.0/12", want: []string{"10.0.0.0/24", "172.16.0.0/12"}},
		{value: "fd00::5/64,\n2001:db8::1", want: []string{"fd00::/64", "2001:db8::1/128"}},
		{value: "::ffff:10.0.0.2", want: []string{"10.0.0.2/32"}},
		{value: "::ffff:10.0.0.0/120", want: []string{"10.0.0.0/24"}},
	}

	for _, tt := range tests {
		got, err := parseTrustedProxies(tt.value)
		if err != nil {
			t.Fatalf("parseTrustedProxies(%q): %v", tt.value, err)
		}
		gotStrings := make([]string, len(got))
		for i, prefix := range got {
			gotStrings[i] = prefix.String()
		}
		if len(got) == 0 && tt.want == nil {
			continue
		}
		if !slices.Equal(gotStrings, tt.want) {
			t.Fatalf("parseTrustedProxies(%q) = %v, want %v", tt.value, gotStrings, tt.want)
		}
	}
}

func TestParseTrustedProxiesRejectsBadEntries(t *testing.T) {
	for _, value := range []string{
		"proxy.local",
		"127.0.0.1, nonsense",
		"10.0.0.0/33",
		"fe80::1%eth0",
		"::ffff:0:0/80",
	} {
		_, err := parseTrustedProxies(value)
		if err == nil {
			t.Fatalf("parseTrustedProxies(%q) succeeded, want an error", value)
		}
		if !strings.Contains(err.Error(), envTrustedProxies) {
			t.Fatalf("parseTrustedProxies(%q) error %q does not name %s", value, err, envTrustedProxies)
		}
	}
}

func TestNewRuntimeConfig_TrustedProxies(t *testing.T) {
	clearRuntimeConfigEnv(t)
	t.Setenv(envTrustedProxies, "127.0.0.1, 10.0.0.0/8")

	config, err := NewRuntimeConfig()
	if err != nil {
		t.Fatalf("NewRuntimeConfig: %v", err)
	}
	if len(config.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies = %v, want 2 prefixes", config.TrustedProxies)
	}

	t.Setenv(envTrustedProxies, "nonsense")
	if _, err := NewRuntimeConfig(); err == nil || !strings.Contains(err.Error(), "nonsense") {
		t.Fatalf("NewRuntimeConfig error = %v, want one naming the bad entry", err)
	}
}
