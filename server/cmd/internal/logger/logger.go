package logger

import (
	"fmt"
	"igloo/cmd/internal/helpers"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type LoggerConfig struct {
	Debug   bool
	LogDir  string
	LogFile string
}

// New creates a new slog.Logger based on the provided configuration.
// In debug mode (Debug=true), logs are written to stdout with text format at debug level.
// In production mode (Debug=false), logs are written to a file with JSON format at info level.
// The log file is automatically rotated to never exceed maxLines.
// Returns the logger, a cleanup function to close the log file, and any error.
// The log directory must exist - this function will not create it.
func New(cfg *LoggerConfig) (*slog.Logger, func() error, error) {
	var w io.Writer
	var closer func() error = func() error { return nil }

	if cfg.Debug {
		w = os.Stdout
		handler := slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})

		return slog.New(handler), closer, nil
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

	return slog.New(handler), closer, nil
}
