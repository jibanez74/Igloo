package logger

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

const loggerFlushInterval = time.Second

// rotatingWriter keeps recent slog JSON entries in a bounded pair of files:
// the live log plus one rotated predecessor (path + ".1").
//
// Writes are buffered and flushed by a background ticker, on severe records
// (see flushOnSevereHandler), and on Close — not per line. Request logging
// sits on the HTTP hot path, and a write syscall per log line dominated the
// server's idle disk writes. The trade-off is that a hard crash can lose up
// to a second of buffered INFO lines.
type rotatingWriter struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	buf      *bufio.Writer
	size     int64
	maxBytes int64
	closed   bool
	stop     chan struct{}
	done     chan struct{}
}

func newRotatingWriter(path string, maxBytes int64) (*rotatingWriter, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("max bytes must be positive")
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	w := &rotatingWriter{
		path:     path,
		file:     f,
		buf:      bufio.NewWriter(f),
		size:     info.Size(),
		maxBytes: maxBytes,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	go w.flushLoop()

	return w, nil
}

func (w *rotatingWriter) flushLoop() {
	defer close(w.done)

	ticker := time.NewTicker(loggerFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.mu.Lock()
			if !w.closed {
				// A flush failure here has nowhere to be reported; the next
				// Write surfaces the buffered writer's sticky error instead.
				_ = w.buf.Flush()
			}
			w.mu.Unlock()
		}
	}
}

// Write implements io.Writer. Each slog entry is one JSON line.
func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, fmt.Errorf("log writer is closed")
	}

	// A single entry larger than the cap is still written whole; the next
	// entry rotates it away.
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	n, err := w.buf.Write(p)
	if err != nil {
		return n, err
	}

	w.size += int64(n)

	return n, nil
}

// rotate renames the live file to its ".1" sibling (replacing any previous
// one) and starts a fresh live file, so rotation cost is O(1) instead of
// rewriting retained lines.
func (w *rotatingWriter) rotate() error {
	err := w.buf.Flush()
	if err != nil {
		return err
	}

	err = w.file.Close()
	if err != nil {
		return err
	}

	err = os.Rename(w.path, w.path+".1")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	w.file = f
	w.buf = bufio.NewWriter(f)
	w.size = 0

	return nil
}

// Flush forces buffered entries to disk; severe records use it so warnings
// and errors never sit in the buffer.
func (w *rotatingWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("log writer is closed")
	}

	return w.buf.Flush()
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()

	if w.closed {
		w.mu.Unlock()
		return fmt.Errorf("log writer is already closed")
	}
	w.closed = true

	flushErr := w.buf.Flush()
	closeErr := w.file.Close()
	w.mu.Unlock()

	close(w.stop)
	<-w.done

	if flushErr != nil {
		return flushErr
	}
	return closeErr
}
