package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const loggerMaxBytes = 1 << 20

type LoggerInterface interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type LoggerConfig struct {
	Debug   bool
	Stdout  bool
	LogDir  string
	LogFile string
}

// New creates a configured logger and cleanup function. Debug selects the
// level and Stdout the sink, independently, so LOG_TO_STDOUT=false keeps
// debug records in the file (as .env.example documents). File logging requires
// an existing LogDir.
func New(cfg *LoggerConfig) (LoggerInterface, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("logger config is required")
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}

	if cfg.Stdout {
		// Debug runs are read by a person in a terminal, so they get text;
		// everything else stays machine-readable.
		var handler slog.Handler
		if cfg.Debug {
			handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
		} else {
			handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
		}

		return slog.New(handler), func() error { return nil }, nil
	}

	logFile := cfg.LogFile
	if logFile == "" {
		logFile = "app.log"
	}

	if cfg.LogDir == "" {
		return nil, nil, fmt.Errorf("log directory is required when stdout logging is disabled")
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

	rw, err := newRotatingWriter(path, loggerMaxBytes, loggerFlushInterval)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return slog.New(newFileHandler(rw, level)), rw.Close, nil
}

// newFileHandler writes JSON records to rw and flushes it after severe ones.
func newFileHandler(rw *rotatingWriter, level slog.Level) slog.Handler {
	handler := slog.NewJSONHandler(rw, &slog.HandlerOptions{
		Level: level,
	})

	return flushOnSevereHandler{Handler: handler, flush: rw.Flush}
}

// flushOnSevereHandler flushes the rotating writer after WARN and ERROR
// records, so severe entries reach disk immediately while routine request
// logging stays buffered.
type flushOnSevereHandler struct {
	slog.Handler
	flush func() error
}

func (h flushOnSevereHandler) Handle(ctx context.Context, record slog.Record) error {
	err := h.Handler.Handle(ctx, record)
	if record.Level >= slog.LevelWarn {
		flushErr := h.flush()
		if err == nil {
			err = flushErr
		}
	}
	return err
}

func (h flushOnSevereHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return flushOnSevereHandler{Handler: h.Handler.WithAttrs(attrs), flush: h.flush}
}

func (h flushOnSevereHandler) WithGroup(name string) slog.Handler {
	return flushOnSevereHandler{Handler: h.Handler.WithGroup(name), flush: h.flush}
}
