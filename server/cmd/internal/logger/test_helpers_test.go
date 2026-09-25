package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// idleFlushInterval keeps the background ticker out of a test's way, so only
// the test's own writes, flushes and closes decide what reaches the file.
const idleFlushInterval = time.Hour

// newTestWriter creates a rotatingWriter over a temp file, seeding the file
// first when seed is not empty. The writer is closed at cleanup, so a test
// that closes it itself is fine: the second close only reports an error.
func newTestWriter(tb testing.TB, maxBytes int64, seed string, flushInterval time.Duration) (*rotatingWriter, string) {
	tb.Helper()

	path := filepath.Join(tb.TempDir(), "test.log")

	if seed != "" {
		err := os.WriteFile(path, []byte(seed), 0o644)
		if err != nil {
			tb.Fatalf("seed log file: %v", err)
		}
	}

	rw, err := newRotatingWriter(path, maxBytes, flushInterval)
	if err != nil {
		tb.Fatalf("create rotating writer: %v", err)
	}

	tb.Cleanup(func() {
		rw.Close()
	})

	return rw, path
}

// failOpen makes openLogFile return the error fail picks for each open flag
// set, deferring to os.OpenFile when it picks nil, until the test ends.
func failOpen(t *testing.T, fail func(flag int) error) {
	t.Helper()

	t.Cleanup(func() { openLogFile = os.OpenFile })
	openLogFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		err := fail(flag)
		if err != nil {
			return nil, err
		}
		return os.OpenFile(name, flag, perm)
	}
}

// requireWriterRecovered proves a writer is usable again after a failed
// rotation: a new entry is accepted and reaches the end of the live file.
func requireWriterRecovered(t *testing.T, rw *rotatingWriter, path string) {
	t.Helper()

	_, err := rw.Write([]byte("recovered\n"))
	if err != nil {
		t.Fatalf("write after a failed rotation: %v", err)
	}

	err = rw.Flush()
	if err != nil {
		t.Fatalf("flush after a failed rotation: %v", err)
	}

	lines := readLogLines(t, path)
	if len(lines) == 0 || lines[len(lines)-1] != "recovered" {
		t.Fatalf("live file = %q, want it to end with the recovered entry", lines)
	}
}

// writeRegularFile creates a regular file and returns its path, for the cases
// that point a directory setting at a file.
func writeRegularFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "notadir")

	err := os.WriteFile(path, []byte("test"), 0o644)
	if err != nil {
		t.Fatalf("write regular file: %v", err)
	}

	return path
}

// readLogLines returns the log file's lines with the trailing newline stripped.
func readLogLines(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	trimmed := strings.TrimSuffix(string(data), "\n")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "\n")
}

// captureStdout swaps os.Stdout for a pipe, runs fn, and returns everything fn
// wrote. New builds its handlers around the value of os.Stdout at construction
// time, so fn has to include the New call itself.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = writer

	t.Cleanup(func() {
		os.Stdout = original
	})

	// Drain concurrently so fn can never block on a full pipe buffer.
	captured := make(chan string, 1)

	go func() {
		data, err := io.ReadAll(reader)
		if err != nil {
			captured <- ""
			return
		}

		captured <- string(data)
	}()

	fn()

	os.Stdout = original

	err = writer.Close()
	if err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	output := <-captured

	err = reader.Close()
	if err != nil {
		t.Fatalf("close pipe reader: %v", err)
	}

	return output
}
