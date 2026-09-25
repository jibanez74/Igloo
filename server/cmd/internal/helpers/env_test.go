package helpers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvFileLoadsSupportedSyntax(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	content := strings.Join([]string{
		"",
		"# full-line comment",
		"SIMPLE=value",
		" export SPACED = trimmed ",
		"exportGLUED=prefix kept",
		"SINGLE='one # two'",
		"SINGLE_COMMENT='kept' # dropped",
		"DOUBLE=\"line\\nquote\\\"slash\\\\return\\r\"",
		"DOUBLE_COMMENT=\"kept\" # dropped",
		"UNKNOWN_ESCAPE=\"tab\\there\"",
		"INLINE=keep # remove",
		"HASH=keep#literal",
		"LEADING_HASH=#literal",
		"DUPLICATE=first",
		"DUPLICATE=second",
		"EMPTY=",
		"",
	}, "\n")
	err := os.WriteFile(envPath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write env file: %v", err)
	}

	keys := []string{
		"SIMPLE",
		"SPACED",
		"exportGLUED",
		"SINGLE",
		"SINGLE_COMMENT",
		"DOUBLE",
		"DOUBLE_COMMENT",
		"UNKNOWN_ESCAPE",
		"INLINE",
		"HASH",
		"LEADING_HASH",
		"DUPLICATE",
		"EMPTY",
	}
	for _, key := range keys {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	err = LoadEnvFile(envPath)
	if err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	assertEnvValue(t, "SIMPLE", "value")
	assertEnvValue(t, "SPACED", "trimmed")
	assertEnvValue(t, "exportGLUED", "prefix kept")
	assertEnvValue(t, "SINGLE", "one # two")
	assertEnvValue(t, "SINGLE_COMMENT", "kept")
	assertEnvValue(t, "DOUBLE", "line\nquote\"slash\\return\r")
	assertEnvValue(t, "DOUBLE_COMMENT", "kept")
	assertEnvValue(t, "UNKNOWN_ESCAPE", "tab\\there")
	assertEnvValue(t, "INLINE", "keep")
	assertEnvValue(t, "HASH", "keep#literal")
	assertEnvValue(t, "LEADING_HASH", "#literal")
	assertEnvValue(t, "DUPLICATE", "second")
	assertEnvValue(t, "EMPTY", "")
}

func TestLoadEnvFileDoesNotOverwriteExistingProcessEnv(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	err := os.WriteFile(envPath, []byte("EXISTING=file\n"), 0o600)
	if err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv("EXISTING", "process")

	err = LoadEnvFile(envPath)
	if err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	assertEnvValue(t, "EXISTING", "process")
}

// Every rejection names the file and the line, so an operator can find the
// offending entry in a long .env.
func TestLoadEnvFileRejectsInvalidLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLine int
		wantErr  string
	}{
		{name: "missing equals", content: "INVALID\n", wantLine: 1, wantErr: "invalid env line"},
		{name: "bare export", content: "export\n", wantLine: 1, wantErr: "invalid env line"},
		{name: "missing key", content: "=value\n", wantLine: 1, wantErr: "missing env key"},
		{name: "whitespace in key", content: "BAD KEY=value\n", wantLine: 1, wantErr: `invalid env key "BAD KEY"`},
		{name: "unterminated single", content: "KEY='value\n", wantLine: 1, wantErr: "unterminated single-quoted env value"},
		{name: "unterminated double", content: "KEY=\"value\n", wantLine: 1, wantErr: "unterminated double-quoted env value"},
		{name: "trailing single-quoted text", content: "KEY='value' trailing\n", wantLine: 1, wantErr: "unexpected text after single-quoted env value"},
		{name: "trailing double-quoted text", content: "KEY=\"value\" trailing\n", wantLine: 1, wantErr: "unexpected text after double-quoted env value"},
		{name: "later line is reported by number", content: "OK=1\n\nINVALID\n", wantLine: 3, wantErr: "invalid env line"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envPath := filepath.Join(t.TempDir(), ".env")
			err := os.WriteFile(envPath, []byte(tt.content), 0o600)
			if err != nil {
				t.Fatalf("write env file: %v", err)
			}

			err = LoadEnvFile(envPath)
			if err == nil {
				t.Fatal("expected LoadEnvFile to fail")
			}

			want := fmt.Sprintf("%s:%d: %s", envPath, tt.wantLine, tt.wantErr)
			if err.Error() != want {
				t.Fatalf("LoadEnvFile error = %q, want %q", err, want)
			}
		})
	}
}

func TestLoadEnvFileReportsAMissingFile(t *testing.T) {
	err := LoadEnvFile(filepath.Join(t.TempDir(), "missing.env"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadEnvFile error = %v, want a not-exist error", err)
	}
}

// bufio.Scanner refuses lines past its token limit, and that failure must not
// be mistaken for an empty or fully loaded file.
func TestLoadEnvFileReportsAnOverlongLine(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	content := "LONG=" + strings.Repeat("x", 70_000) + "\n"
	err := os.WriteFile(envPath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write env file: %v", err)
	}

	err = LoadEnvFile(envPath)
	if err == nil {
		t.Fatal("expected LoadEnvFile to fail")
	}
	if !strings.HasPrefix(err.Error(), "read "+envPath+": ") {
		t.Fatalf("LoadEnvFile error = %q, want it to report the read failure", err)
	}
}

// A key the OS refuses is reported as such rather than silently skipped.
func TestLoadEnvFileReportsSetenvFailure(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	err := os.WriteFile(envPath, []byte("BAD\x00KEY=value\n"), 0o600)
	if err != nil {
		t.Fatalf("write env file: %v", err)
	}

	err = LoadEnvFile(envPath)
	if err == nil {
		t.Fatal("expected LoadEnvFile to fail")
	}
	if !strings.HasPrefix(err.Error(), "set env BAD\x00KEY: ") {
		t.Fatalf("LoadEnvFile error = %q, want it to report the set failure", err)
	}
}

func assertEnvValue(t *testing.T, key string, want string) {
	t.Helper()

	got, ok := os.LookupEnv(key)
	if !ok {
		t.Fatalf("expected %s to be set", key)
	}
	if got != want {
		t.Fatalf("expected %s=%q, got %q", key, want, got)
	}
}
