// Package fakebin writes POSIX shell scripts that stand in for external
// binaries such as ffmpeg and ffprobe in tests, and reads back the argument
// logs those scripts record.
//
// It imports only the standard library, so the in-package tests of the
// packages it fakes can use it without an import cycle. Production code never
// imports it.
package fakebin

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// header starts every script: fail on the first error or unset variable.
const header = "#!/bin/sh\nset -eu\n"

// Write writes header+body as an executable named name in a fresh
// t.TempDir() and returns its path. A missing trailing newline is added.
func Write(t testing.TB, name, body string) string {
	t.Helper()
	contents := header + body
	if !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	path := filepath.Join(t.TempDir(), name)
	err := os.WriteFile(path, []byte(contents), 0o755)
	if err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
	return path
}

// Spec describes a fake that records its arguments, prints fixed output and
// exits with a fixed code.
type Spec struct {
	Stdout   string // printed to stdout with a trailing newline
	Stderr   string // printed to stderr with a trailing newline
	ExitCode int
	ArgsLog  string // when set, see ArgsLogLine
	CallLog  string // when set, see CallLogLine
}

// WriteSpec writes the fake spec describes; see Write.
func WriteSpec(t testing.TB, name string, spec Spec) string {
	t.Helper()
	var body strings.Builder
	if spec.ArgsLog != "" {
		body.WriteString(ArgsLogLine(spec.ArgsLog))
	}
	if spec.CallLog != "" {
		body.WriteString(CallLogLine(spec.CallLog))
	}
	if spec.Stdout != "" {
		body.WriteString("printf '%s\\n' " + Quote(spec.Stdout) + "\n")
	}
	if spec.Stderr != "" {
		body.WriteString("printf '%s\\n' " + Quote(spec.Stderr) + " >&2\n")
	}
	if spec.ExitCode != 0 {
		body.WriteString("exit " + strconv.Itoa(spec.ExitCode) + "\n")
	}
	return Write(t, name, body.String())
}

// ArgsLogLine returns a script line that overwrites logPath with one line per
// argument, for a fake called once whose individual arguments are asserted.
func ArgsLogLine(logPath string) string {
	return "printf '%s\\n' \"$@\" > " + Quote(logPath) + "\n"
}

// CallLogLine returns a script line that appends one line per invocation,
// holding the whole argument list, for a fake called several times.
func CallLogLine(logPath string) string {
	return "printf '%s\\n' \"$*\" >> " + Quote(logPath) + "\n"
}

// Quote single-quotes value for the shell.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// ReadLog returns the lines of a log written by ArgsLogLine or CallLogLine.
// The log is trimmed first, so an empty log reads as [""] and whitespace at
// the very start or end of the logged arguments is dropped.
func ReadLog(t testing.TB, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read argument log: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// CountLine returns how many lines of the log at path equal line, such as
// the "-version" invocations in a CallLogLine log.
func CountLine(t testing.TB, path, line string) int {
	t.Helper()
	count := 0
	for _, got := range ReadLog(t, path) {
		if got == line {
			count++
		}
	}
	return count
}
