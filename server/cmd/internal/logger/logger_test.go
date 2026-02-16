package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
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

		// Closer should be a no-op but not nil
		if closer == nil {
			t.Fatal("expected closer to be non-nil")
		}

		// Calling closer should not error
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

		// Log file should be created
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
			LogFile: "", // Empty - should default to app.log
		}

		logger, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer closer()

		if logger == nil {
			t.Fatal("expected logger to be non-nil")
		}

		// Default log file should be app.log
		logPath := filepath.Join(dir, "app.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("expected default log file 'app.log' to be created")
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

		// Create a file where we expect a directory
		filePath := filepath.Join(dir, "notadir")
		err := os.WriteFile(filePath, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		cfg := &LoggerConfig{
			Debug:  false,
			LogDir: filePath, // This is a file, not a directory
		}

		_, _, err = New(cfg)
		if err == nil {
			t.Fatal("expected error when log path is a file")
		}

		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("expected error about not being a directory, got %v", err)
		}
	})

	t.Run("logger writes to file in production mode", func(t *testing.T) {
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

		// Write a log message
		logger.Info("test message", "key", "value")

		// Close to flush
		err = closer()
		if err != nil {
			t.Fatalf("closer failed: %v", err)
		}

		// Read the log file
		content, err := os.ReadFile(filepath.Join(dir, "test.log"))
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		// Should contain JSON with our message
		if !strings.Contains(string(content), "test message") {
			t.Errorf("expected log file to contain 'test message', got %s", string(content))
		}

		if !strings.Contains(string(content), `"key":"value"`) {
			t.Errorf("expected log file to contain key-value pair, got %s", string(content))
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

		// First close should succeed
		err = closer()
		if err != nil {
			t.Errorf("first close should succeed, got %v", err)
		}

		// Second close should fail (file already closed)
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
			LogDir: "", // Empty but should be fine in debug mode
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
			LogFile: "", // Empty but should be fine in debug mode
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

