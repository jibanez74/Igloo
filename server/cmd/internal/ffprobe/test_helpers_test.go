package ffprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeFFprobeSpec describes a fake ffprobe binary: what it prints, how it
// exits, and whether it records its arguments.
type fakeFFprobeSpec struct {
	stdout   string // printed to stdout with a trailing newline
	stderr   string // printed to stderr with a trailing newline
	exitCode int
	argsLog  string // when set, the script writes one line per argument here
}

func writeFakeFFprobe(t *testing.T, spec fakeFFprobeSpec) string {
	t.Helper()

	var body strings.Builder
	body.WriteString("#!/bin/sh\n")
	if spec.argsLog != "" {
		body.WriteString("printf '%s\\n' \"$@\" > " + shellQuote(spec.argsLog) + "\n")
	}
	if spec.stdout != "" {
		body.WriteString("printf '%s\\n' " + shellQuote(spec.stdout) + "\n")
	}
	if spec.stderr != "" {
		body.WriteString("printf '%s\\n' " + shellQuote(spec.stderr) + " >&2\n")
	}
	if spec.exitCode != 0 {
		body.WriteString("exit " + strconv.Itoa(spec.exitCode) + "\n")
	}

	binPath := filepath.Join(t.TempDir(), "ffprobe")
	err := os.WriteFile(binPath, []byte(body.String()), 0755)
	if err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	return binPath
}

func readArgumentLog(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read argument log: %v", err)
	}

	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func requireArgumentValue(t *testing.T, args []string, flag string, want string) {
	t.Helper()

	for index, arg := range args {
		if arg != flag {
			continue
		}
		if index+1 >= len(args) {
			t.Fatalf("%s has no value", flag)
		}
		if args[index+1] != want {
			t.Fatalf("%s = %q, want %q", flag, args[index+1], want)
		}
		return
	}

	t.Fatalf("%s argument not found in %q", flag, args)
}

func shellQuote(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(value, "'", "'\\''"))
}
