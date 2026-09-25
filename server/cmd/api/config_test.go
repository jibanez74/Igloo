package main

import (
	"os"
	"path/filepath"
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
		envJellyfinAPIKey,
		envSpotifyClientID,
		envSpotifyClientSecret,
		envHardwareAccelerationDevice,
		envEnableWatcher,
		envDownloadImages,
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
	old, hadOld := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if hadOld {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})

	envFile, loaded, err := LoadRuntimeEnvFile()
	if err != nil {
		t.Fatalf("LoadRuntimeEnvFile failed: %v", err)
	}

	if !loaded || envFile != ".env" {
		t.Fatalf("expected .env to be loaded, got file=%q loaded=%t", envFile, loaded)
	}
	if got := os.Getenv(key); got != "loaded" {
		t.Fatalf("expected env file value to load, got %q", got)
	}
}

func TestLoadRuntimeEnvFile_MissingEnvFileIsIgnored(t *testing.T) {
	changeWorkingDirectory(t, t.TempDir())

	envFile, loaded, err := LoadRuntimeEnvFile()
	if err != nil {
		t.Fatalf("LoadRuntimeEnvFile failed: %v", err)
	}
	if loaded || envFile != "" {
		t.Fatalf("expected missing .env to be ignored, got file=%q loaded=%t", envFile, loaded)
	}
}
