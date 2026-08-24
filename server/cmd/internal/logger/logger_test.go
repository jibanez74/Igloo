package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			name:      "debug mode writes text records at debug level",
			config:    LoggerConfig{Debug: true},
			wantText:  true,
			wantDebug: true,
		},
		{
			name:   "stdout mode writes json records at info level",
			config: LoggerConfig{Stdout: true},
		},
		{
			name:      "debug mode wins over stdout mode",
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
