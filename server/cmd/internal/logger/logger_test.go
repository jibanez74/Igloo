package logger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  func(t *testing.T) *LoggerConfig
		wantErr string
	}{
		{
			name:    "nil config",
			config:  func(t *testing.T) *LoggerConfig { return nil },
			wantErr: "logger config is required",
		},
		{
			name:    "empty log directory",
			config:  func(t *testing.T) *LoggerConfig { return &LoggerConfig{} },
			wantErr: "log directory is required",
		},
		{
			name: "log directory does not exist",
			config: func(t *testing.T) *LoggerConfig {
				return &LoggerConfig{LogDir: filepath.Join(t.TempDir(), "missing")}
			},
			wantErr: "log directory does not exist",
		},
		{
			name: "log directory is a regular file",
			config: func(t *testing.T) *LoggerConfig {
				return &LoggerConfig{LogDir: writeRegularFile(t)}
			},
			wantErr: "log path is not a directory",
		},
		{
			name: "log directory sits under a regular file",
			config: func(t *testing.T) *LoggerConfig {
				// Stat fails with ENOTDIR here, which is not a not-exist error.
				return &LoggerConfig{LogDir: filepath.Join(writeRegularFile(t), "nested")}
			},
			wantErr: "failed to stat log directory",
		},
		{
			name: "log file path is taken by a directory",
			config: func(t *testing.T) *LoggerConfig {
				dir := t.TempDir()

				err := os.Mkdir(filepath.Join(dir, "app.log"), 0o755)
				if err != nil {
					t.Fatalf("create directory: %v", err)
				}

				return &LoggerConfig{LogDir: dir}
			},
			wantErr: "failed to open log file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, closer, err := New(tt.config(t))
			if err == nil {
				t.Fatal("expected an error")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}

			if logger != nil {
				t.Error("expected a nil logger on failure")
			}

			if closer != nil {
				t.Error("expected a nil closer on failure")
			}
		})
	}
}

func TestNewFileLogger(t *testing.T) {
	t.Run("defaults the file name without mutating the config", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &LoggerConfig{LogDir: dir}

		_, closer, err := New(cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		defer closer()

		_, err = os.Stat(filepath.Join(dir, "app.log"))
		if err != nil {
			t.Fatalf("expected app.log to be created: %v", err)
		}

		if cfg.LogFile != "" {
			t.Errorf("cfg.LogFile = %q, want it to stay empty", cfg.LogFile)
		}
	})

	t.Run("writes info records as json and filters debug records", func(t *testing.T) {
		dir := t.TempDir()

		logger, closer, err := New(&LoggerConfig{LogDir: dir, LogFile: "test.log"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		logger.Info("test message", "key", "value")
		logger.Debug("debug message", "key", "hidden")

		err = closer()
		if err != nil {
			t.Fatalf("closer: %v", err)
		}

		lines := readLogLines(t, filepath.Join(dir, "test.log"))
		if len(lines) != 1 {
			t.Fatalf("got %d lines, want only the info record: %q", len(lines), lines)
		}

		if !strings.Contains(lines[0], `"msg":"test message"`) {
			t.Errorf("line = %q, want the info message", lines[0])
		}

		if !strings.Contains(lines[0], `"key":"value"`) {
			t.Errorf("line = %q, want the info attributes", lines[0])
		}
	})

	t.Run("debug mode keeps file logging and records debug", func(t *testing.T) {
		dir := t.TempDir()

		logger, closer, err := New(&LoggerConfig{Debug: true, LogDir: dir, LogFile: "test.log"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		logger.Info("info message")
		logger.Debug("debug message")

		err = closer()
		if err != nil {
			t.Fatalf("closer: %v", err)
		}

		lines := readLogLines(t, filepath.Join(dir, "test.log"))
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want the info and debug records: %q", len(lines), lines)
		}

		if !strings.Contains(lines[1], `"level":"DEBUG"`) || !strings.Contains(lines[1], `"msg":"debug message"`) {
			t.Errorf("line = %q, want the debug record as json", lines[1])
		}
	})

	t.Run("closer reports a repeated close", func(t *testing.T) {
		_, closer, err := New(&LoggerConfig{LogDir: t.TempDir(), LogFile: "test.log"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		err = closer()
		if err != nil {
			t.Fatalf("first close: %v", err)
		}

		err = closer()
		if err == nil {
			t.Error("expected an error on the second close")
		}
	})
}

func TestNewStdoutLogger(t *testing.T) {
	tests := []struct {
		name      string
		config    LoggerConfig
		wantText  bool
		wantDebug bool
	}{
		{
			name:   "stdout mode writes json records at info level",
			config: LoggerConfig{Stdout: true},
		},
		{
			name:      "debug mode writes text records at debug level",
			config:    LoggerConfig{Debug: true, Stdout: true},
			wantText:  true,
			wantDebug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// LogDir stays empty: neither stdout path needs a log directory.
			cfg := tt.config

			var closer func() error
			var newErr error

			output := captureStdout(t, func() {
				logger, c, err := New(&cfg)
				closer, newErr = c, err
				if err != nil {
					return
				}

				logger.Info("info message")
				logger.Debug("debug message")
			})

			if newErr != nil {
				t.Fatalf("New: %v", newErr)
			}

			if closer == nil {
				t.Fatal("expected a non-nil closer")
			}

			err := closer()
			if err != nil {
				t.Errorf("closer: %v", err)
			}

			if !strings.Contains(output, "info message") {
				t.Fatalf("output = %q, want the info record", output)
			}

			isText := strings.Contains(output, "level=INFO")
			if isText != tt.wantText {
				t.Errorf("text handler = %v, want %v (output %q)", isText, tt.wantText, output)
			}

			if !tt.wantText && !strings.Contains(output, `"level":"INFO"`) {
				t.Errorf("output = %q, want json records", output)
			}

			hasDebug := strings.Contains(output, "debug message")
			if hasDebug != tt.wantDebug {
				t.Errorf("debug record = %v, want %v (output %q)", hasDebug, tt.wantDebug, output)
			}
		})
	}
}

// countingHandler records what the wrapped handler saw and can fail on demand,
// so the severe-flush hook's ordering and error precedence are observable.
type countingHandler struct {
	slog.Handler
	handleErr error
}

func (h countingHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.handleErr != nil {
		return h.handleErr
	}
	return h.Handler.Handle(ctx, record)
}

func TestFlushOnSevereHandler(t *testing.T) {
	newHandler := func(handleErr, flushErr error) (flushOnSevereHandler, *int, *bytes.Buffer) {
		flushes := 0
		var buf bytes.Buffer
		wrapped := countingHandler{Handler: slog.NewJSONHandler(&buf, nil), handleErr: handleErr}
		handler := flushOnSevereHandler{
			Handler: wrapped,
			flush: func() error {
				flushes++
				return flushErr
			},
		}
		return handler, &flushes, &buf
	}

	t.Run("flushes warn and error records but not info", func(t *testing.T) {
		handler, flushes, _ := newHandler(nil, nil)
		logger := slog.New(handler)

		logger.Info("routine")
		if *flushes != 0 {
			t.Fatalf("flushes after Info = %d, want 0", *flushes)
		}

		logger.Warn("severe")
		if *flushes != 1 {
			t.Fatalf("flushes after Warn = %d, want 1", *flushes)
		}

		logger.Error("severe")
		if *flushes != 2 {
			t.Fatalf("flushes after Error = %d, want 2", *flushes)
		}
	})

	t.Run("derived handlers keep the hook", func(t *testing.T) {
		handler, flushes, buf := newHandler(nil, nil)
		logger := slog.New(handler)

		logger.With("request", "abc").Warn("with attrs")
		if *flushes != 1 {
			t.Fatalf("flushes after With().Warn = %d, want 1", *flushes)
		}
		if !strings.Contains(buf.String(), `"request":"abc"`) {
			t.Errorf("record = %q, want the attribute from With", buf.String())
		}

		logger.WithGroup("scan").Error("with group", "state", "failed")
		if *flushes != 2 {
			t.Fatalf("flushes after WithGroup().Error = %d, want 2", *flushes)
		}
		if !strings.Contains(buf.String(), `"scan":{"state":"failed"}`) {
			t.Errorf("record = %q, want the grouped attribute", buf.String())
		}
	})

	t.Run("reports a flush failure only when the record was written", func(t *testing.T) {
		handleErr := errors.New("handle failed")
		flushErr := errors.New("flush failed")

		handler, flushes, _ := newHandler(nil, flushErr)
		err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelWarn, "severe", 0))
		if !errors.Is(err, flushErr) {
			t.Errorf("Handle error = %v, want the flush error", err)
		}

		handler, flushes, _ = newHandler(handleErr, flushErr)
		err = handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelError, "severe", 0))
		if !errors.Is(err, handleErr) {
			t.Errorf("Handle error = %v, want the handler's own error to win", err)
		}
		if *flushes != 1 {
			t.Errorf("flushes = %d, want the flush to still run after a failed handle", *flushes)
		}
	})
}

// Routine records sit in the writer's buffer until the ticker, a severe record
// or the closer flushes them; a warning must be on disk before any of those.
func TestNewFileLoggerFlushesSevereRecordsImmediately(t *testing.T) {
	dir := t.TempDir()

	logger, closer, err := New(&LoggerConfig{LogDir: dir, LogFile: "test.log"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { closer() })

	path := filepath.Join(dir, "test.log")

	logger.Info("buffered")
	lines := readLogLines(t, path)
	if len(lines) != 0 {
		t.Fatalf("lines after Info = %q, want the record to stay buffered", lines)
	}

	logger.Warn("flushed")
	lines = readLogLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("lines after Warn = %q, want both records on disk", lines)
	}
	if !strings.Contains(lines[1], `"level":"WARN"`) {
		t.Errorf("line = %q, want the warning record", lines[1])
	}
}
