package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewRotatingWriter(t *testing.T) {
	t.Run("creates a new file", func(t *testing.T) {
		rw, path := newTestWriter(t, 100, "")

		if rw.lines != 0 {
			t.Errorf("lines = %d, want 0", rw.lines)
		}

		if rw.maxLines != 100 {
			t.Errorf("maxLines = %d, want 100", rw.maxLines)
		}

		_, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected the log file to be created: %v", err)
		}
	})

	t.Run("seeds the line count from an existing file", func(t *testing.T) {
		rw, _ := newTestWriter(t, 100, "line1\nline2\nline3\nline4\nline5\n")

		if rw.lines != 5 {
			t.Errorf("lines = %d, want 5", rw.lines)
		}
	})

	t.Run("rejects a non positive max line count", func(t *testing.T) {
		_, err := newRotatingWriter(filepath.Join(t.TempDir(), "test.log"), 0)
		if err == nil {
			t.Fatal("expected an error")
		}

		if !strings.Contains(err.Error(), "max lines must be positive") {
			t.Errorf("error = %v, want a max lines error", err)
		}
	})

	t.Run("rejects an unopenable path", func(t *testing.T) {
		_, err := newRotatingWriter(filepath.Join(t.TempDir(), "missing", "test.log"), 100)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "empty file", content: "", want: 0},
		{name: "newline terminated lines", content: "line1\nline2\nline3\n", want: 3},
		{name: "unterminated final line", content: "line1\nline2", want: 2},
		{name: "line longer than the read buffer", content: longLine() + "\nshort\n", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := countLines(openSeededFile(t, tt.content))
			if err != nil {
				t.Fatalf("countLines: %v", err)
			}

			if count != tt.want {
				t.Errorf("count = %d, want %d", count, tt.want)
			}
		})
	}

	t.Run("reports the seek failure for a closed file", func(t *testing.T) {
		f := openSeededFile(t, "line1\n")

		err := f.Close()
		if err != nil {
			t.Fatalf("close: %v", err)
		}

		_, err = countLines(f)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestRotatingWriter_Write(t *testing.T) {
	t.Run("reports the written byte count", func(t *testing.T) {
		rw, _ := newTestWriter(t, 100, "")

		entry := []byte("test log line\n")

		n, err := rw.Write(entry)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		if n != len(entry) {
			t.Errorf("n = %d, want %d", n, len(entry))
		}

		if rw.lines != 1 {
			t.Errorf("lines = %d, want 1", rw.lines)
		}
	})

	t.Run("counts one line per write", func(t *testing.T) {
		rw, _ := newTestWriter(t, 100, "")

		for i := 0; i < 10; i++ {
			_, err := rw.Write([]byte("log line\n"))
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		}

		if rw.lines != 10 {
			t.Errorf("lines = %d, want 10", rw.lines)
		}
	})

	t.Run("flushes on every write", func(t *testing.T) {
		rw, path := newTestWriter(t, 100, "")

		_, err := rw.Write([]byte("persisted line\n"))
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		// Read while the writer is still open: entries have to survive a crash.
		lines := readLogLines(t, path)
		if len(lines) != 1 || lines[0] != "persisted line" {
			t.Errorf("lines = %q, want [persisted line]", lines)
		}
	})
}

func TestRotatingWriter_Rotate(t *testing.T) {
	t.Run("rotates once the max line count is reached", func(t *testing.T) {
		maxLines := 10
		rw, _ := newTestWriter(t, maxLines, "")

		for i := 0; i < maxLines; i++ {
			_, err := rw.Write([]byte("line\n"))
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		}

		if rw.lines != maxLines {
			t.Fatalf("lines = %d, want %d before the rotation trigger", rw.lines, maxLines)
		}

		_, err := rw.Write([]byte("trigger rotation\n"))
		if err != nil {
			t.Fatalf("rotation write: %v", err)
		}

		want := (maxLines / 2) + 1
		if rw.lines != want {
			t.Errorf("lines = %d, want %d after rotation", rw.lines, want)
		}
	})

	t.Run("keeps the newest half", func(t *testing.T) {
		maxLines := 6
		rw, path := newTestWriter(t, maxLines, "")

		for i := 1; i <= maxLines; i++ {
			_, err := rw.Write([]byte(strings.Repeat("x", i) + "\n"))
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		}

		_, err := rw.Write([]byte("final\n"))
		if err != nil {
			t.Fatalf("final write: %v", err)
		}

		want := []string{"xxxx", "xxxxx", "xxxxxx", "final"}

		lines := readLogLines(t, path)
		if strings.Join(lines, "|") != strings.Join(want, "|") {
			t.Errorf("lines = %q, want %q", lines, want)
		}
	})

	t.Run("keeps a retained line longer than the read buffer", func(t *testing.T) {
		long := longLine()
		rw, path := newTestWriter(t, 3, "old\n"+long+"\nnewer\n")

		_, err := rw.Write([]byte("final\n"))
		if err != nil {
			t.Fatalf("final write: %v", err)
		}

		lines := readLogLines(t, path)
		if len(lines) != 3 {
			t.Fatalf("got %d lines, want 3", len(lines))
		}

		if lines[0] != long {
			t.Errorf("expected the long line to be retained intact, got %d bytes", len(lines[0]))
		}

		if lines[1] != "newer" || lines[2] != "final" {
			t.Errorf("lines = %q, want the newest entries last", lines[1:])
		}
	})

	t.Run("rotates repeatedly", func(t *testing.T) {
		rw, path := newTestWriter(t, 4, "")

		for i := 1; i <= 12; i++ {
			_, err := rw.Write([]byte(fmt.Sprintf("line%02d\n", i)))
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		}

		// Each rotation drops the oldest half, so three rotations leave the
		// last four entries behind.
		want := []string{"line09", "line10", "line11", "line12"}

		lines := readLogLines(t, path)
		if strings.Join(lines, "|") != strings.Join(want, "|") {
			t.Errorf("lines = %q, want %q", lines, want)
		}

		if rw.lines != len(want) {
			t.Errorf("lines counter = %d, want %d", rw.lines, len(want))
		}
	})
}

func TestRotatingWriter_Close(t *testing.T) {
	t.Run("closes a writer with no writes", func(t *testing.T) {
		rw, _ := newTestWriter(t, 100, "")

		err := rw.Close()
		if err != nil {
			t.Errorf("close: %v", err)
		}
	})

	t.Run("write after close reports the flush failure", func(t *testing.T) {
		rw, _ := newTestWriter(t, 100, "")

		err := rw.Close()
		if err != nil {
			t.Fatalf("close: %v", err)
		}

		_, err = rw.Write([]byte("after close\n"))
		if err == nil {
			t.Fatal("expected an error writing to a closed writer")
		}

		if strings.Contains(err.Error(), "failed to rotate log file") {
			t.Errorf("error = %v, want a flush failure rather than a rotation failure", err)
		}

		// The buffered writer stays failed rather than quietly accepting more.
		_, err = rw.Write([]byte("after close\n"))
		if err == nil {
			t.Error("expected the writer to keep reporting the failure")
		}

		if rw.lines != 0 {
			t.Errorf("lines = %d, want failed writes not to be counted", rw.lines)
		}
	})

	t.Run("write after close reports the rotation failure", func(t *testing.T) {
		rw, _ := newTestWriter(t, 2, "line1\nline2\n")

		err := rw.Close()
		if err != nil {
			t.Fatalf("close: %v", err)
		}

		_, err = rw.Write([]byte("after close\n"))
		if err == nil {
			t.Fatal("expected an error writing to a closed writer")
		}

		if !strings.Contains(err.Error(), "failed to rotate log file") {
			t.Errorf("error = %v, want a rotation failure", err)
		}
	})
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	const (
		goroutines = 5
		perRoutine = 20
		maxLines   = 20
	)

	rw, path := newTestWriter(t, maxLines, "")

	errs := make(chan error, goroutines*perRoutine)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < perRoutine; j++ {
				_, err := rw.Write([]byte("concurrent write\n"))
				if err != nil {
					errs <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent write: %v", err)
	}

	// Interleaving changes nothing observable: the counter has to keep matching
	// the file, and rotation has to keep the file inside its bounds.
	lines := readLogLines(t, path)
	if rw.lines != len(lines) {
		t.Errorf("lines counter = %d, but the file holds %d lines", rw.lines, len(lines))
	}

	if rw.lines <= maxLines/2 || rw.lines > maxLines {
		t.Errorf("lines = %d, want it within (%d, %d]", rw.lines, maxLines/2, maxLines)
	}
}
