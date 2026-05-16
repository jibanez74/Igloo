package logger

import (
	"fmt"
	"igloo/cmd/internal/helpers"
	"io"
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

type logger struct {
	*slog.Logger
}

var _ LoggerInterface = (*logger)(nil)

type LoggerConfig struct {
	Debug   bool
	Stdout  bool
	LogDir  string
	LogFile string
}

// New creates a configured logger and cleanup function.
// Production file logging requires an existing LogDir.
func New(cfg *LoggerConfig) (LoggerInterface, func() error, error) {
	var w io.Writer
	var closer func() error = func() error { return nil }

	if cfg.Stdout {
		w = os.Stdout

		if cfg.Debug {
			handler := slog.NewTextHandler(w, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})

			return &logger{Logger: slog.New(handler)}, closer, nil
		}

		handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})

		return &logger{Logger: slog.New(handler)}, closer, nil
	}

	if cfg.Debug {
		w = os.Stdout
		handler := slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})

		return &logger{Logger: slog.New(handler)}, closer, nil
	}

	if cfg.LogFile == "" {
		cfg.LogFile = "app.log"
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

	path := filepath.Join(cfg.LogDir, cfg.LogFile)

	rw, err := newRotatingWriter(path, helpers.LOGGER_MAX_LINES)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	w = rw
	closer = rw.Close

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return &logger{Logger: slog.New(handler)}, closer, nil
}
