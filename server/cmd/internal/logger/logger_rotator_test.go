package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewRotatingWriter(t *testing.T) {
	t.Run("seeds the size from an existing file", func(t *testing.T) {
		seed := "line1\nline2\n"
		rw, _ := newTestWriter(t, 1024, seed, idleFlushInterval)

		if rw.size != int64(len(seed)) {
			t.Errorf("size = %d, want %d", rw.size, len(seed))
		}
	})

	t.Run("rejects a non positive byte cap", func(t *testing.T) {
		_, err := newRotatingWriter(filepath.Join(t.TempDir(), "test.log"), 0, loggerFlushInterval)
		if err == nil {
			t.Fatal("expected an error")
		}

		if !strings.Contains(err.Error(), "max bytes must be positive") {
			t.Errorf("error = %v, want a max bytes error", err)
		}
	})

	t.Run("rejects a non positive flush interval", func(t *testing.T) {
		_, err := newRotatingWriter(filepath.Join(t.TempDir(), "test.log"), 1024, 0)
		if err == nil {
			t.Fatal("expected an error")
		}

		if !strings.Contains(err.Error(), "flush interval must be positive") {
			t.Errorf("error = %v, want a flush interval error", err)
		}
	})
}

func TestRotatingWriter_Write(t *testing.T) {
	t.Run("reports the written byte count", func(t *testing.T) {
		rw, _ := newTestWriter(t, 1024, "", idleFlushInterval)

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
		rw, path := newTestWriter(t, 1024, "", idleFlushInterval)

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
		rw, path := newTestWriter(t, 1024, "", idleFlushInterval)

		_, err := rw.Write([]byte("closed line\n"))
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

	t.Run("entries reach the file on the flush tick", func(t *testing.T) {
		rw, path := newTestWriter(t, 1024, "", 5*time.Millisecond)

		_, err := rw.Write([]byte("ticked line\n"))
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		deadline := time.Now().Add(2 * time.Second)
		for {
			lines := readLogLines(t, path)
			if len(lines) == 1 && lines[0] == "ticked line" {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("lines = %q after waiting for the flush tick, want [ticked line]", lines)
			}
			time.Sleep(time.Millisecond)
		}
	})

	t.Run("reports a failed write", func(t *testing.T) {
		rw, _ := newTestWriter(t, loggerMaxBytes, "", idleFlushInterval)

		// Closing the file underneath the writer makes the buffered writer's
		// own write fail; an entry larger than its buffer bypasses the buffer
		// and hits the file directly.
		err := rw.file.Close()
		if err != nil {
			t.Fatalf("close underlying file: %v", err)
		}

		_, err = rw.Write([]byte(strings.Repeat("x", 8192) + "\n"))
		if err == nil {
			t.Fatal("expected an error writing to a closed file")
		}
	})
}

func TestRotatingWriter_Rotate(t *testing.T) {
	t.Run("rotates once the byte cap would be exceeded", func(t *testing.T) {
		entry := []byte("0123456789\n")
		rw, path := newTestWriter(t, int64(3*len(entry)), "", idleFlushInterval)

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
		rw, path := newTestWriter(t, int64(2*len(entry)), "", idleFlushInterval)

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
		rw, path := newTestWriter(t, 8, "", idleFlushInterval)

		big := strings.Repeat("x", 16)
		_, err := rw.Write([]byte(big + "\n"))
		if err != nil {
			t.Fatalf("oversized write: %v", err)
		}

		err = rw.Flush()
		if err != nil {
			t.Fatalf("flush: %v", err)
		}

		lines := readLogLines(t, path)
		if len(lines) != 1 || lines[0] != big {
			t.Errorf("expected the oversized entry to be written whole")
		}

		_, err = os.Stat(path + ".1")
		if !os.IsNotExist(err) {
			t.Errorf("expected no rotation before the oversized entry, stat err: %v", err)
		}
	})

	// A rotation that fails after the live file was closed used to leave the
	// writer over that closed file, so every later entry failed too and the
	// process logged nothing for the rest of its life.
	t.Run("keeps logging after a rotation fails", func(t *testing.T) {
		seed := "retained\n"
		rw, path := newTestWriter(t, int64(len(seed)), seed, idleFlushInterval)

		// A directory in the rotated file's place makes the rename fail.
		err := os.Mkdir(path+".1", 0o755)
		if err != nil {
			t.Fatalf("block the rotated path: %v", err)
		}

		_, err = rw.Write([]byte("lost\n"))
		if err == nil {
			t.Fatal("expected the write that triggers rotation to report the failure")
		}
		if !strings.Contains(err.Error(), "failed to rotate log file") {
			t.Errorf("error = %v, want a rotation error", err)
		}

		_, err = rw.Write([]byte("after failure\n"))
		if err != nil {
			t.Fatalf("write after a failed rotation: %v", err)
		}

		err = rw.Flush()
		if err != nil {
			t.Fatalf("flush: %v", err)
		}

		lines := readLogLines(t, path)
		want := []string{"retained", "after failure"}
		if strings.Join(lines, "|") != strings.Join(want, "|") {
			t.Errorf("live file = %q, want the retained line followed by the new entry %q", lines, want)
		}
	})

	// Each failure below strikes after the live file is no longer usable: a
	// flush into a closed descriptor, a close of one, a rename into a blocked
	// path (above), or an open that fails after the rename. Every one must
	// end with the writer over a reopened live file.
	t.Run("keeps logging after a failed flush or close", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			buffered bool
		}{
			{name: "flush", buffered: true},
			{name: "close", buffered: false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rw, path := newTestWriter(t, 8, "", idleFlushInterval)

				_, err := rw.Write([]byte("buffered\n"))
				if err != nil {
					t.Fatalf("write: %v", err)
				}
				if !tc.buffered {
					// With nothing buffered the rotation's flush succeeds, so
					// the close of the dead descriptor is what fails.
					err = rw.Flush()
					if err != nil {
						t.Fatalf("flush: %v", err)
					}
				}

				err = rw.file.Close()
				if err != nil {
					t.Fatalf("close underlying file: %v", err)
				}

				_, err = rw.Write([]byte("trigger\n"))
				if err == nil || !strings.Contains(err.Error(), "failed to rotate log file") {
					t.Fatalf("error = %v, want a rotation error", err)
				}

				requireWriterRecovered(t, rw, path)
			})
		}
	})

	t.Run("keeps logging when the fresh live file cannot be created", func(t *testing.T) {
		rw, path := newTestWriter(t, 8, "", idleFlushInterval)
		errCreate := errors.New("create refused")
		failOpen(t, func(flag int) error {
			if flag&os.O_TRUNC != 0 {
				return errCreate
			}
			return nil
		})

		_, err := rw.Write([]byte("rotated\n"))
		if err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err = rw.Write([]byte("trigger\n"))
		if !errors.Is(err, errCreate) {
			t.Fatalf("error = %v, want the create failure", err)
		}

		// The rename went through, so the old entry sits in the rotated file
		// and the reopened live file starts empty.
		rotated := readLogLines(t, path+".1")
		if len(rotated) != 1 || rotated[0] != "rotated" {
			t.Errorf("rotated file = %q, want [rotated]", rotated)
		}
		requireWriterRecovered(t, rw, path)
	})

	t.Run("reports both errors when the reopen fails too", func(t *testing.T) {
		rw, _ := newTestWriter(t, 8, "", idleFlushInterval)
		errCreate := errors.New("create refused")
		errReopen := errors.New("reopen refused")
		failOpen(t, func(flag int) error {
			if flag&os.O_TRUNC != 0 {
				return errCreate
			}
			return errReopen
		})

		_, err := rw.Write([]byte("rotated\n"))
		if err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err = rw.Write([]byte("trigger\n"))
		if !errors.Is(err, errCreate) || !errors.Is(err, errReopen) {
			t.Fatalf("error = %v, want both the create and the reopen failure", err)
		}
	})
}

func TestRotatingWriter_Close(t *testing.T) {
	t.Run("write and flush after close report errors", func(t *testing.T) {
		rw, _ := newTestWriter(t, 1024, "", idleFlushInterval)

		err := rw.Close()
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

	t.Run("reports buffered entries it could not write out", func(t *testing.T) {
		rw, _ := newTestWriter(t, 1024, "", idleFlushInterval)

		_, err := rw.Write([]byte("buffered\n"))
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		err = rw.file.Close()
		if err != nil {
			t.Fatalf("close underlying file: %v", err)
		}

		err = rw.Close()
		if err == nil {
			t.Fatal("expected close to report the failed flush")
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

	rw, path := newTestWriter(t, maxBytes, "", idleFlushInterval)

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

	rotatedInfo, err := os.Stat(path + ".1")
	if err != nil {
		t.Fatalf("stat rotated log: %v", err)
	}

	if rotatedInfo.Size() > maxBytes {
		t.Errorf("rotated file = %d bytes, want it within the %d byte cap", rotatedInfo.Size(), maxBytes)
	}
}
