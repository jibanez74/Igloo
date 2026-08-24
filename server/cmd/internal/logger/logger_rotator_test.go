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
		rw, path := newTestWriter(t, 1024, "")

		if rw.size != 0 {
			t.Errorf("size = %d, want 0", rw.size)
		}

		if rw.maxBytes != 1024 {
			t.Errorf("maxBytes = %d, want 1024", rw.maxBytes)
		}

		_, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected the log file to be created: %v", err)
		}
	})

	t.Run("seeds the size from an existing file", func(t *testing.T) {
		seed := "line1\nline2\n"
		rw, _ := newTestWriter(t, 1024, seed)

		if rw.size != int64(len(seed)) {
			t.Errorf("size = %d, want %d", rw.size, len(seed))
		}
	})

	t.Run("rejects a non positive byte cap", func(t *testing.T) {
		_, err := newRotatingWriter(filepath.Join(t.TempDir(), "test.log"), 0)
		if err == nil {
			t.Fatal("expected an error")
		}

		if !strings.Contains(err.Error(), "max bytes must be positive") {
			t.Errorf("error = %v, want a max bytes error", err)
		}
	})

	t.Run("rejects an unopenable path", func(t *testing.T) {
		_, err := newRotatingWriter(filepath.Join(t.TempDir(), "missing", "test.log"), 1024)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestRotatingWriter_Write(t *testing.T) {
	t.Run("reports the written byte count", func(t *testing.T) {
		rw, _ := newTestWriter(t, 1024, "")

		entry := []byte("test log line\n")

		n, err := rw.Write(entry)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		if n != len(entry) {
			t.Errorf("n = %d, want %d", n, len(entry))
		}

		if rw.size != int64(len(entry)) {
			t.Errorf("size = %d, want %d", rw.size, len(entry))
		}
	})

	t.Run("entries reach the file after a flush", func(t *testing.T) {
		rw, path := newTestWriter(t, 1024, "")

		_, err := rw.Write([]byte("buffered line\n"))
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		err = rw.Flush()
		if err != nil {
			t.Fatalf("flush: %v", err)
		}

		lines := readLogLines(t, path)
		if len(lines) != 1 || lines[0] != "buffered line" {
			t.Errorf("lines = %q, want [buffered line]", lines)
		}
	})

	t.Run("entries reach the file on close", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.log")

		rw, err := newRotatingWriter(path, 1024)
		if err != nil {
			t.Fatalf("create rotating writer: %v", err)
		}

		_, err = rw.Write([]byte("closed line\n"))
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		err = rw.Close()
		if err != nil {
			t.Fatalf("close: %v", err)
		}

		lines := readLogLines(t, path)
		if len(lines) != 1 || lines[0] != "closed line" {
			t.Errorf("lines = %q, want [closed line]", lines)
		}
	})
}

func TestRotatingWriter_Rotate(t *testing.T) {
	t.Run("rotates once the byte cap would be exceeded", func(t *testing.T) {
		entry := []byte("0123456789\n")
		rw, path := newTestWriter(t, int64(3*len(entry)), "")

		for i := 0; i < 3; i++ {
			_, err := rw.Write(entry)
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		}

		_, err := rw.Write([]byte("trigger\n"))
		if err != nil {
			t.Fatalf("rotation write: %v", err)
		}

		err = rw.Flush()
		if err != nil {
			t.Fatalf("flush: %v", err)
		}

		lines := readLogLines(t, path)
		if len(lines) != 1 || lines[0] != "trigger" {
			t.Errorf("live file = %q, want only the post-rotation entry", lines)
		}

		rotated := readLogLines(t, path+".1")
		if len(rotated) != 3 {
			t.Errorf("rotated file holds %d lines, want the 3 pre-rotation entries", len(rotated))
		}
	})

	t.Run("keeps at most two generations", func(t *testing.T) {
		entry := []byte("aaaaaaaaa\n")
		rw, path := newTestWriter(t, int64(2*len(entry)), "")

		for i := 1; i <= 9; i++ {
			_, err := rw.Write([]byte(fmt.Sprintf("line%04d0\n", i)))
			if err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
		}

		err := rw.Flush()
		if err != nil {
			t.Fatalf("flush: %v", err)
		}

		live := readLogLines(t, path)
		rotated := readLogLines(t, path+".1")

		if len(live)+len(rotated) != 3 {
			t.Errorf("retained %d lines across generations, want 3", len(live)+len(rotated))
		}

		all := append(rotated, live...)
		want := []string{"line00070", "line00080", "line00090"}
		if strings.Join(all, "|") != strings.Join(want, "|") {
			t.Errorf("retained = %q, want the newest entries %q", all, want)
		}

		_, err = os.Stat(path + ".2")
		if !os.IsNotExist(err) {
			t.Errorf("expected no third generation, stat err: %v", err)
		}
	})

	t.Run("writes an entry larger than the cap without rotating first", func(t *testing.T) {
		rw, path := newTestWriter(t, 8, "")

		big := longLine() + "\n"
		_, err := rw.Write([]byte(big))
		if err != nil {
			t.Fatalf("oversized write: %v", err)
		}

		err = rw.Flush()
		if err != nil {
			t.Fatalf("flush: %v", err)
		}

		lines := readLogLines(t, path)
		if len(lines) != 1 || lines[0] != longLine() {
			t.Errorf("expected the oversized entry to be written whole")
		}
	})
}

func TestRotatingWriter_Close(t *testing.T) {
	t.Run("closes a writer with no writes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.log")

		rw, err := newRotatingWriter(path, 1024)
		if err != nil {
			t.Fatalf("create rotating writer: %v", err)
		}

		err = rw.Close()
		if err != nil {
			t.Errorf("close: %v", err)
		}
	})

	t.Run("repeated close reports an error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.log")

		rw, err := newRotatingWriter(path, 1024)
		if err != nil {
			t.Fatalf("create rotating writer: %v", err)
		}

		err = rw.Close()
		if err != nil {
			t.Fatalf("first close: %v", err)
		}

		err = rw.Close()
		if err == nil {
			t.Error("expected an error on the second close")
		}
	})

	t.Run("write and flush after close report errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.log")

		rw, err := newRotatingWriter(path, 1024)
		if err != nil {
			t.Fatalf("create rotating writer: %v", err)
		}

		err = rw.Close()
		if err != nil {
			t.Fatalf("close: %v", err)
		}

		_, err = rw.Write([]byte("after close\n"))
		if err == nil {
			t.Error("expected an error writing to a closed writer")
		}

		err = rw.Flush()
		if err == nil {
			t.Error("expected an error flushing a closed writer")
		}
	})
}

func TestRotatingWriter_ConcurrentWrites(t *testing.T) {
	const (
		goroutines = 5
		perRoutine = 20
	)

	entry := "concurrent write\n"
	maxBytes := int64(20 * len(entry))

	rw, path := newTestWriter(t, maxBytes, "")

	errs := make(chan error, goroutines*perRoutine)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < perRoutine; j++ {
				_, err := rw.Write([]byte(entry))
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

	err := rw.Flush()
	if err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Interleaving changes nothing observable: both generations stay inside
	// the byte cap and the live file matches the size counter.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat live log: %v", err)
	}

	if info.Size() != rw.size {
		t.Errorf("size counter = %d, but the file holds %d bytes", rw.size, info.Size())
	}

	if info.Size() > maxBytes {
		t.Errorf("live file = %d bytes, want it within the %d byte cap", info.Size(), maxBytes)
	}
}
