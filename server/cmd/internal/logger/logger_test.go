package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("returns error for nil config", func(t *testing.T) {
		_, _, err := New(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}

		if !strings.Contains(err.Error(), "logger config is required") {
			t.Errorf("expected config required error, got %v", err)
		}
	})

	t.Run("debug mode returns stdout logger", func(t *testing.T) {
		cfg := &LoggerConfig{
			Debug: true,
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if logger == nil {
			t.Fatal("expected logger to be non-nil")
		}

		if closer == nil {
			t.Fatal("expected closer to be non-nil")
		}

		err = closer()
		if err != nil {
			t.Errorf("expected closer to not error, got %v", err)
		}
	})

	t.Run("production mode creates file logger", func(t *testing.T) {
		dir := t.TempDir()

		cfg := &LoggerConfig{
			Debug:   false,
			LogDir:  dir,
			LogFile: "test.log",
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer closer()

		if logger == nil {
			t.Fatal("expected logger to be non-nil")
		}

		logPath := filepath.Join(dir, "test.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("expected log file to be created")
		}
	})

	t.Run("production mode uses default log file name", func(t *testing.T) {
		dir := t.TempDir()

		cfg := &LoggerConfig{
			Debug:   false,
			LogDir:  dir,
			LogFile: "",
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer closer()

		if logger == nil {
			t.Fatal("expected logger to be non-nil")
		}

		logPath := filepath.Join(dir, "app.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("expected default log file 'app.log' to be created")
		}

		if cfg.LogFile != "" {
			t.Errorf("expected config log file to stay empty, got %q", cfg.LogFile)
		}
	})

	t.Run("stdout mode does not require log directory", func(t *testing.T) {
		cfg := &LoggerConfig{
			Stdout: true,
			LogDir: "",
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer closer()

		if logger == nil {
			t.Fatal("expected logger to be non-nil")
		}
	})

	t.Run("returns error when log directory is empty in production mode", func(t *testing.T) {
		cfg := &LoggerConfig{
			Debug:  false,
			LogDir: "",
		}

		_, _, err := New(cfg)
		if err == nil {
			t.Fatal("expected error for empty log directory")
		}

		if !strings.Contains(err.Error(), "log directory is required") {
			t.Errorf("expected error about log directory required, got %v", err)
		}
	})

	t.Run("returns error when log directory does not exist", func(t *testing.T) {
		cfg := &LoggerConfig{
			Debug:  false,
			LogDir: "/nonexistent/directory/path",
		}

		_, _, err := New(cfg)
		if err == nil {
			t.Fatal("expected error for nonexistent log directory")
		}

		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected error about directory not existing, got %v", err)
		}
	})

	t.Run("returns error when log path is a file not directory", func(t *testing.T) {
		dir := t.TempDir()

		filePath := filepath.Join(dir, "notadir")
		err := os.WriteFile(filePath, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		cfg := &LoggerConfig{
			Debug:  false,
			LogDir: filePath,
		}

		_, _, err = New(cfg)
		if err == nil {
			t.Fatal("expected error when log path is a file")
		}

		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("expected error about not being a directory, got %v", err)
		}
	})

	t.Run("production logger writes info and filters debug records", func(t *testing.T) {
		dir := t.TempDir()

		cfg := &LoggerConfig{
			Debug:   false,
			LogDir:  dir,
			LogFile: "test.log",
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		logger.Info("test message", "key", "value")
		logger.Debug("debug message", "key", "hidden")

		err = closer()
		if err != nil {
			t.Fatalf("closer failed: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(dir, "test.log"))
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		if !strings.Contains(string(content), "test message") {
			t.Errorf("expected log file to contain 'test message', got %s", string(content))
		}

		if !strings.Contains(string(content), `"key":"value"`) {
			t.Errorf("expected log file to contain key-value pair, got %s", string(content))
		}

		if strings.Contains(string(content), "debug message") {
			t.Errorf("expected debug log to be filtered, got %s", string(content))
		}
	})

	t.Run("closer properly closes file", func(t *testing.T) {
		dir := t.TempDir()

		cfg := &LoggerConfig{
			Debug:   false,
			LogDir:  dir,
			LogFile: "test.log",
		}

		_, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		err = closer()
		if err != nil {
			t.Errorf("first close should succeed, got %v", err)
		}

		err = closer()
		if err == nil {
			t.Error("expected error on second close (file already closed)")
		}
	})
}

func TestLoggerConfig(t *testing.T) {
	t.Run("debug mode ignores log directory", func(t *testing.T) {
		cfg := &LoggerConfig{
			Debug:  true,
			LogDir: "",
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error in debug mode with empty dir, got %v", err)
		}
		defer closer()

		if logger == nil {
			t.Fatal("expected logger to be non-nil")
		}
	})

	t.Run("debug mode ignores log file", func(t *testing.T) {
		cfg := &LoggerConfig{
			Debug:   true,
			LogFile: "",
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error in debug mode with empty file, got %v", err)
		}
		defer closer()

		if logger == nil {
			t.Fatal("expected logger to be non-nil")
		}
	})
}
