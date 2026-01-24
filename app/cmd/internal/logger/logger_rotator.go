package logger

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// rotatingWriter is an io.Writer implementation that wraps a file and ensures
// it never exceeds a maximum number of lines. When the limit is reached,
// it discards the oldest half of the log entries and keeps the newest half.
// This prevents unbounded log file growth while preserving recent history.
// The writer is thread-safe and uses buffered I/O for efficient writes.
// Each log entry from slog is one line (JSON format), so line count equals entry count.
type rotatingWriter struct {
	path     string        // Full path to the log file
	file     *os.File      // Open file handle for read/write operations
	buf      *bufio.Writer // Buffered writer for efficient disk writes
	mu       sync.Mutex    // Protects all fields for concurrent access
	lines    int           // Current number of lines in the file
	maxLines int           // Maximum allowed lines before rotation
}

// newRotatingWriter creates a new rotating writer for the given file path.
// It opens (or creates) the log file and counts existing lines to initialize
// the line counter. The maxLines parameter sets when rotation will occur.
// Returns an error if the file cannot be opened or read.
func newRotatingWriter(path string, maxLines int) (*rotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	// Count existing lines so we know when to rotate
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

// countLines reads the file from the beginning and counts the number of lines.
// This is called once at startup to initialize the line counter.
func countLines(f *os.File) (int, error) {
	// Seek to the beginning of the file
	_, err := f.Seek(0, 0)
	if err != nil {
		return 0, err
	}

	count := 0
	scanner := bufio.NewScanner(f)

	// Count each line in the file
	for scanner.Scan() {
		count++
	}

	return count, scanner.Err()
}

// Write implements io.Writer. It writes data to the log file with rotation support.
// If the line limit is reached, rotation occurs before writing the new entry.
// Thread-safe: multiple goroutines can safely call Write concurrently.
// Each Write call is assumed to be one complete log line (slog writes one JSON line per log).
func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we need to rotate before writing
	if w.lines >= w.maxLines {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	// Write to the buffer (fast, in-memory operation)
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
// This strategy:
//   - Keeps enough history to be useful for debugging
//   - Avoids rotating on every single write after hitting the limit
//   - Provides O(n) rotation but only runs every ~250 writes (for 500 line limit)
//
// Process:
//  1. Flush any buffered writes
//  2. Read all lines from the file
//  3. Keep only the newest 50% of lines
//  4. Truncate and rewrite the file with kept lines
func (w *rotatingWriter) rotate() error {
	// Ensure any pending writes are flushed before we read the file
	err := w.buf.Flush()
	if err != nil {
		return err
	}

	// Seek to beginning to read all lines
	_, err = w.file.Seek(0, 0)
	if err != nil {
		return err
	}

	// Pre-allocate slice with known capacity to avoid reallocations during scan
	lines := make([]string, 0, w.lines)
	scanner := bufio.NewScanner(w.file)

	// Read all existing lines into memory
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	err = scanner.Err()
	if err != nil {
		return err
	}

	// Keep only the newest half of lines.
	// Example: 500 lines -> keep lines 250-499 (newest 250 entries)
	keepFrom := len(lines) / 2
	lines = lines[keepFrom:]

	// Truncate the file to zero length
	err = w.file.Truncate(0)
	if err != nil {
		return err
	}

	// Seek back to beginning for writing
	_, err = w.file.Seek(0, 0)
	if err != nil {
		return err
	}

	// Reset the buffer to point to the file's new position (start)
	w.buf.Reset(w.file)

	// Build all retained lines into a single string for efficient writing.
	// Using strings.Builder avoids repeated allocations from string concatenation.
	// Pre-grow to estimated size (~100 bytes per JSON log line) to minimize reallocations.
	var builder strings.Builder
	builder.Grow(len(lines) * 100)

	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	// Write all lines in a single operation (efficient: one syscall after buffer fills)
	_, err = w.buf.WriteString(builder.String())
	if err != nil {
		return err
	}

	// Flush to ensure all data is persisted to disk
	err = w.buf.Flush()
	if err != nil {
		return err
	}

	// Update line count to reflect the new state
	w.lines = len(lines)

	return nil
}

// Close flushes any buffered data and closes the underlying file.
// Should be called during application shutdown to ensure no logs are lost.
func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush any remaining buffered data
	err := w.buf.Flush()
	if err != nil {
		// Still close the file even if flush fails
		w.file.Close()

		return err
	}

	return w.file.Close()
}
