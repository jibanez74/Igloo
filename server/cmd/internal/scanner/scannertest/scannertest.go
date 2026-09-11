// Package scannertest holds the test doubles the movie and music scanner
// suites share: a level-capturing logger, a row counter, and ffprobe stubs.
package scannertest

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"

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
// needle in any of its structured values.
func (l *Logger) WarnMentions(msg, needle string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.WarnEntries {
		if entry.Msg != msg {
			continue
		}
		for _, arg := range entry.Args {
			value, ok := arg.(string)
			if ok && strings.Contains(value, needle) {
				return true
			}
		}
	}
	return false
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
