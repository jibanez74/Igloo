package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRotatingWriter(t *testing.T) {
	t.Run("creates new file if it does not exist", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer rw.Close()

		if rw.lines != 0 {
			t.Errorf("expected 0 lines, got %d", rw.lines)
		}

		if rw.maxLines != 100 {
			t.Errorf("expected maxLines 100, got %d", rw.maxLines)
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("expected file to be created")
		}
	})

	t.Run("counts existing lines in file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		// Create file with 5 lines
		content := "line1\nline2\nline3\nline4\nline5\n"
		err := os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer rw.Close()

		if rw.lines != 5 {
			t.Errorf("expected 5 lines, got %d", rw.lines)
		}
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		// Try to create in a non-existent directory
		path := "/nonexistent/directory/test.log"

		_, err := newRotatingWriter(path, 100)
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

func TestRotatingWriter_Write(t *testing.T) {
	t.Run("writes single line", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}
		defer rw.Close()

		n, err := rw.Write([]byte("test log line\n"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if n != 14 {
			t.Errorf("expected 14 bytes written, got %d", n)
		}

		if rw.lines != 1 {
			t.Errorf("expected 1 line, got %d", rw.lines)
		}
	})

	t.Run("writes multiple lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}
		defer rw.Close()

		for i := 0; i < 10; i++ {
			_, err := rw.Write([]byte("log line\n"))
			if err != nil {
				t.Fatalf("write %d failed: %v", i, err)
			}
		}

		if rw.lines != 10 {
			t.Errorf("expected 10 lines, got %d", rw.lines)
		}
	})

	t.Run("persists data to disk", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}

		_, err = rw.Write([]byte("persisted line\n"))
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}

		rw.Close()

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		if string(content) != "persisted line\n" {
			t.Errorf("expected 'persisted line\\n', got %q", string(content))
		}
	})
}

func TestRotatingWriter_Rotate(t *testing.T) {
	t.Run("rotates when max lines reached", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		maxLines := 10
		rw, err := newRotatingWriter(path, maxLines)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}
		defer rw.Close()

		// Write exactly maxLines
		for i := 0; i < maxLines; i++ {
			_, err := rw.Write([]byte("line\n"))
			if err != nil {
				t.Fatalf("write %d failed: %v", i, err)
			}
		}

		// At this point we have 10 lines, next write should trigger rotation
		if rw.lines != maxLines {
			t.Errorf("expected %d lines before rotation trigger, got %d", maxLines, rw.lines)
		}

		// Write one more to trigger rotation
		_, err = rw.Write([]byte("trigger rotation\n"))
		if err != nil {
			t.Fatalf("rotation write failed: %v", err)
		}

		// After rotation: kept 5 lines (half of 10) + 1 new = 6
		expectedLines := (maxLines / 2) + 1
		if rw.lines != expectedLines {
			t.Errorf("expected %d lines after rotation, got %d", expectedLines, rw.lines)
		}
	})

	t.Run("keeps newest lines after rotation", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		maxLines := 6
		rw, err := newRotatingWriter(path, maxLines)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}

		// Write lines with identifiable content
		for i := 1; i <= maxLines; i++ {
			line := strings.Repeat("x", i) + "\n" // "x", "xx", "xxx", etc.
			_, err := rw.Write([]byte(line))
			if err != nil {
				t.Fatalf("write %d failed: %v", i, err)
			}
		}

		// Trigger rotation with a final line
		_, err = rw.Write([]byte("final\n"))
		if err != nil {
			t.Fatalf("final write failed: %v", err)
		}

		rw.Close()

		// Read and verify content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")

		// Should have kept the newest half (3 lines: "xxxx", "xxxxx", "xxxxxx") + "final"
		if len(lines) != 4 {
			t.Errorf("expected 4 lines after rotation, got %d: %v", len(lines), lines)
		}

		// First kept line should be "xxxx" (the 4th original line)
		if lines[0] != "xxxx" {
			t.Errorf("expected first line to be 'xxxx', got %q", lines[0])
		}

		// Last line should be "final"
		if lines[len(lines)-1] != "final" {
			t.Errorf("expected last line to be 'final', got %q", lines[len(lines)-1])
		}
	})

	t.Run("handles multiple rotations", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		maxLines := 4
		rw, err := newRotatingWriter(path, maxLines)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}
		defer rw.Close()

		// Write 12 lines - should trigger rotation multiple times
		for i := 0; i < 12; i++ {
			_, err := rw.Write([]byte("line\n"))
			if err != nil {
				t.Fatalf("write %d failed: %v", i, err)
			}
		}

		// Should never exceed maxLines (approximately)
		if rw.lines > maxLines {
			t.Errorf("expected lines <= %d, got %d", maxLines, rw.lines)
		}
	})
}

func TestRotatingWriter_Close(t *testing.T) {
	t.Run("flushes data on close", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}

		_, err = rw.Write([]byte("buffered data\n"))
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}

		err = rw.Close()
		if err != nil {
			t.Fatalf("close failed: %v", err)
		}

		// Verify data was flushed
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		if string(content) != "buffered data\n" {
			t.Errorf("expected 'buffered data\\n', got %q", string(content))
		}
	})

	t.Run("can close empty writer", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}

		err = rw.Close()
		if err != nil {
			t.Errorf("expected no error closing empty writer, got %v", err)
		}
	})
}

func TestCountLines(t *testing.T) {
	t.Run("counts zero lines in empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.log")

		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		defer f.Close()

		count, err := countLines(f)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if count != 0 {
			t.Errorf("expected 0 lines, got %d", count)
		}
	})

	t.Run("counts lines correctly", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")

		content := "line1\nline2\nline3\n"
		err := os.WriteFile(path, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		defer f.Close()

		count, err := countLines(f)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if count != 3 {
			t.Errorf("expected 3 lines, got %d", count)
		}
	})
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	t.Run("handles concurrent writes safely", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "concurrent.log")

		rw, err := newRotatingWriter(path, 100)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}
		defer rw.Close()

		// Spawn multiple goroutines writing concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(id int) {
				for j := 0; j < 10; j++ {
					_, err := rw.Write([]byte("concurrent write\n"))
					if err != nil {
						t.Errorf("goroutine %d write %d failed: %v", id, j, err)
					}
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have 100 lines (10 goroutines * 10 writes)
		if rw.lines != 100 {
			t.Errorf("expected 100 lines, got %d", rw.lines)
		}
	})

	t.Run("handles concurrent writes with rotation", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "concurrent_rotate.log")

		maxLines := 20
		rw, err := newRotatingWriter(path, maxLines)
		if err != nil {
			t.Fatalf("failed to create writer: %v", err)
		}
		defer rw.Close()

		// Spawn goroutines that will trigger rotation
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(id int) {
				for j := 0; j < 20; j++ {
					_, err := rw.Write([]byte("concurrent\n"))
					if err != nil {
						t.Errorf("goroutine %d write %d failed: %v", id, j, err)
					}
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 5; i++ {
			<-done
		}

		// Lines should be within reasonable bounds (rotation keeps it from exploding)
		if rw.lines > maxLines {
			t.Errorf("expected lines <= %d after rotations, got %d", maxLines, rw.lines)
		}
	})
}
