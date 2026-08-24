package helpers

import (
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
		"SINGLE='one # two'",
		"DOUBLE=\"line\\nquote\\\"slash\\\\return\\r\"",
		"INLINE=keep # remove",
		"HASH=keep#literal",
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
		"SINGLE",
		"DOUBLE",
		"INLINE",
		"HASH",
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
	assertEnvValue(t, "SINGLE", "one # two")
	assertEnvValue(t, "DOUBLE", "line\nquote\"slash\\return\r")
	assertEnvValue(t, "INLINE", "keep")
	assertEnvValue(t, "HASH", "keep#literal")
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

func TestLoadEnvFileRejectsInvalidLines(t *testing.T) {
	tests := map[string]string{
		"missing equals":       "INVALID\n",
		"missing key":          "=value\n",
		"whitespace in key":    "BAD KEY=value\n",
		"unterminated single":  "KEY='value\n",
		"unterminated double":  "KEY=\"value\n",
		"trailing quoted text": "KEY=\"value\" trailing\n",
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			envPath := filepath.Join(t.TempDir(), ".env")
			err := os.WriteFile(envPath, []byte(content), 0o600)
			if err != nil {
				t.Fatalf("write env file: %v", err)
			}

			err = LoadEnvFile(envPath)
			if err == nil {
				t.Fatal("expected LoadEnvFile to fail")
			}
		})
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
