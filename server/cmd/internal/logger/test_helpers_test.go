package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestWriter creates a rotatingWriter over a temp file, seeding the file
// first when seed is not empty.
func newTestWriter(t *testing.T, maxBytes int64, seed string) (*rotatingWriter, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.log")

	if seed != "" {
		err := os.WriteFile(path, []byte(seed), 0o644)
		if err != nil {
			t.Fatalf("seed log file: %v", err)
		}
	}

	rw, err := newRotatingWriter(path, maxBytes)
	if err != nil {
		t.Fatalf("create rotating writer: %v", err)
	}

	t.Cleanup(func() {
		rw.Close()
	})

	return rw, path
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

// longLine is larger than bufio's default read buffer, which forces the line
// readers to stitch one line back together across several reads.
func longLine() string {
	return strings.Repeat("x", 70*1024)
}
