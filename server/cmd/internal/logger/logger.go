package logger

import (
	"fmt"
	"igloo/cmd/internal/helpers"
	"log/slog"
	"os"
	"path/filepath"
)

type LoggerInterface interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

var _ LoggerInterface = (*slog.Logger)(nil)

type LoggerConfig struct {
	Debug   bool
	Stdout  bool
	LogDir  string
	LogFile string
}

// New creates a configured logger and cleanup function.
// Production file logging requires an existing LogDir.
func New(cfg *LoggerConfig) (LoggerInterface, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("logger config is required")
	}

	closer := func() error { return nil }

	if cfg.Debug {
		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})

		return slog.New(handler), closer, nil
	}

	if cfg.Stdout {
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})

		return slog.New(handler), closer, nil
	}

	logFile := cfg.LogFile
	if logFile == "" {
		logFile = "app.log"
	}

	if cfg.LogDir == "" {
		return nil, nil, fmt.Errorf("log directory is required when debug mode is disabled")
	}

	info, err := os.Stat(cfg.LogDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("log directory does not exist: %s", cfg.LogDir)
		}

		return nil, nil, fmt.Errorf("failed to stat log directory: %w", err)
	}

	if !info.IsDir() {
		return nil, nil, fmt.Errorf("log path is not a directory: %s", cfg.LogDir)
	}

	path := filepath.Join(cfg.LogDir, logFile)

	rw, err := newRotatingWriter(path, helpers.LOGGER_MAX_LINES)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	closer = rw.Close

	handler := slog.NewJSONHandler(rw, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return slog.New(handler), closer, nil
}
