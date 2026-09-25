// Package scannertest holds the test doubles the movie, music, and TV scanner
// suites share: a level-capturing logger, a row counter, database setup, and
// ffprobe stubs.
package scannertest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
)

// LogEntry is one captured log call.
type LogEntry struct {
	Msg  string
	Args []any
}

// Logger records log calls by level for assertions.
type Logger struct {
	mu           sync.Mutex
	DebugEntries []LogEntry
	InfoEntries  []LogEntry
	WarnEntries  []LogEntry
	ErrorEntries []LogEntry
}

func (l *Logger) log(entries *[]LogEntry, msg string, args []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*entries = append(*entries, LogEntry{Msg: msg, Args: append([]any(nil), args...)})
}

func (l *Logger) Debug(msg string, args ...any) { l.log(&l.DebugEntries, msg, args) }
func (l *Logger) Info(msg string, args ...any)  { l.log(&l.InfoEntries, msg, args) }
func (l *Logger) Warn(msg string, args ...any)  { l.log(&l.WarnEntries, msg, args) }
func (l *Logger) Error(msg string, args ...any) { l.log(&l.ErrorEntries, msg, args) }

// WarnMentions reports whether a warning with the given message carries
// needle in any of its structured values. Errors and Stringers are matched
// on their text, so "error", err arguments can be searched too.
func (l *Logger) WarnMentions(msg, needle string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return mentions(l.WarnEntries, msg, needle)
}

// DebugMentions is WarnMentions for debug entries, which is where the
// scanners report deferrals that are not failures.
func (l *Logger) DebugMentions(msg, needle string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return mentions(l.DebugEntries, msg, needle)
}

func mentions(entries []LogEntry, msg, needle string) bool {
	for _, entry := range entries {
		if entry.Msg != msg {
			continue
		}
		for _, arg := range entry.Args {
			if strings.Contains(argText(arg), needle) {
				return true
			}
		}
	}
	return false
}

func argText(arg any) string {
	switch value := arg.(type) {
	case string:
		return value
	case error:
		return value.Error()
	case fmt.Stringer:
		return value.String()
	}
	return ""
}

// WriteFile writes contents to path, creating parent directories, and fails
// the test on error. It returns path so fixtures can be declared inline.
func WriteFile(t testing.TB, path, contents string) string {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	err = os.WriteFile(path, []byte(contents), 0o644)
	if err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// WaitForGroup waits for wait to drain or fails the test after timeout, so a
// scan that never stops reports what it was instead of hanging the package.
func WaitForGroup(t testing.TB, wait *sync.WaitGroup, timeout time.Duration, what string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	WaitForSignal(t, done, timeout, what)
}

// WaitForSignal receives from signal or fails the test after timeout.
func WaitForSignal(t testing.TB, signal <-chan struct{}, timeout time.Duration, what string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(timeout):
		t.Fatalf("%s did not happen within %s", what, timeout)
	}
}

// CountRows runs a single-column count query and fails the test on error.
func CountRows(t testing.TB, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

// NoKeyframeProbe completes ffprobe.FfprobeInterface for stubs that only
// exercise scanning. Keyframe lookup is advisory on the playback path, so a
// stub that never serves HLS refuses it rather than inventing an offset.
type NoKeyframeProbe struct{}

func (NoKeyframeProbe) KeyframeAtOrBefore(context.Context, string, int64, float64) (float64, error) {
	return 0, errors.New("keyframe probing is not stubbed")
}

// Probe answers every metadata request through Callback.
type Probe struct {
	NoKeyframeProbe
	Callback func(context.Context, string) (*ffprobe.FfprobeResult, error)
}

func (p *Probe) GetMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.Callback(ctx, path)
}

func (p *Probe) GetAudioMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.Callback(ctx, path)
}

// CountingProbe counts metadata requests so rescans can assert that unchanged
// files are not re-probed. Hook answers a request when set; otherwise a copy
// of Default is returned. Scanners probe on worker goroutines, so the counter
// is locked.
type CountingProbe struct {
	NoKeyframeProbe
	mu      sync.Mutex
	calls   int
	Hook    func(context.Context, string) (*ffprobe.FfprobeResult, error)
	Default *ffprobe.FfprobeResult
}

func (p *CountingProbe) GetMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	p.mu.Lock()
	p.calls++
	hook := p.Hook
	p.mu.Unlock()
	if hook != nil {
		return hook(ctx, path)
	}
	if p.Default == nil {
		return nil, errors.New("no probe result configured")
	}
	result := *p.Default
	return &result, nil
}

func (p *CountingProbe) GetAudioMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.GetMetadata(ctx, path)
}

func (p *CountingProbe) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}
