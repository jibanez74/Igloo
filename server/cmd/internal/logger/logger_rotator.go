package logger

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// rotatingWriter keeps recent slog JSON entries in a bounded file.
type rotatingWriter struct {
	path     string
	file     *os.File
	buf      *bufio.Writer
	mu       sync.Mutex
	lines    int
	maxLines int
}

func newRotatingWriter(path string, maxLines int) (*rotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	lines, err := countLines(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	return &rotatingWriter{
		path:     path,
		file:     f,
		buf:      bufio.NewWriter(f),
		lines:    lines,
		maxLines: maxLines,
	}, nil
}

func countLines(f *os.File) (int, error) {
	_, err := f.Seek(0, 0)
	if err != nil {
		return 0, err
	}

	count := 0
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		count++
	}

	return count, scanner.Err()
}

// Write implements io.Writer. Each slog entry is one JSON line.
func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.lines >= w.maxLines {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	n, err = w.buf.Write(p)
	if err != nil {
		return n, err
	}

	// Flush immediately to ensure logs survive crashes.
	// Trade-off: slightly slower writes for guaranteed persistence.
	err = w.buf.Flush()
	if err != nil {
		return n, err
	}

	w.lines++

	return n, nil
}

// rotate discards the oldest half of the log file and keeps the newest half.
// Keeping half avoids rotating on every write once the limit is reached.
func (w *rotatingWriter) rotate() error {
	err := w.buf.Flush()
	if err != nil {
		return err
	}

	_, err = w.file.Seek(0, 0)
	if err != nil {
		return err
	}

	lines := make([]string, 0, w.lines)
	scanner := bufio.NewScanner(w.file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	err = scanner.Err()
	if err != nil {
		return err
	}

	keepFrom := len(lines) / 2
	lines = lines[keepFrom:]

	err = w.file.Truncate(0)
	if err != nil {
		return err
	}

	_, err = w.file.Seek(0, 0)
	if err != nil {
		return err
	}

	w.buf.Reset(w.file)

	var builder strings.Builder
	builder.Grow(len(lines) * 100)

	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	_, err = w.buf.WriteString(builder.String())
	if err != nil {
		return err
	}

	err = w.buf.Flush()
	if err != nil {
		return err
	}

	w.lines = len(lines)

	return nil
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.buf.Flush()
	if err != nil {
		// Still close the file even if flush fails
		w.file.Close()

		return err
	}

	return w.file.Close()
}
