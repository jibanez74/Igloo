//go:build externalbin

package movie

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/sqlc"
)

// Opt-in: media is opened read-only; the database always lives in t.TempDir.
// Set IGLOO_BENCH_MOVIES_DIR and IGLOO_FFPROBE_PATH to run against a real library.
func TestMovieLibraryBenchmark(t *testing.T) {
	root := os.Getenv("IGLOO_BENCH_MOVIES_DIR")
	if root == "" {
		t.Skip("set IGLOO_BENCH_MOVIES_DIR for read-only real-library measurements")
	}
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "benchmark.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(sqlc.Schema)
	if err != nil {
		t.Fatal(err)
	}
	queries, err := database.Prepare(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer queries.Close()
	probe, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}
	defer ffprobe.Cleanup()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	log := &measurementLogger{t: t}
	s := New(Dependencies{DB: db, Queries: queries, Ffprobe: probe, Logger: log, ScanContext: ctx})
	for _, run := range []string{"fresh", "recovery", "unchanged"} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		t.Logf("%s initial I/O: %s", run, processMeasurement("/proc/self/io"))
		s.scan(root)
		runtime.ReadMemStats(&after)
		status := s.Status()
		t.Logf("%s: elapsed=%s total=%d processed=%d imported=%d unchanged=%d failed=%d deferred=%d heap=%d allocated=%d", run, time.Since(start), status.Total, status.Processed, status.Imported, status.Unchanged, status.Failed, status.Deferred, after.HeapAlloc, after.TotalAlloc-before.TotalAlloc)
		t.Logf("%s final I/O: %s; memory: %s", run, processMeasurement("/proc/self/io"), processMeasurement("/proc/self/status"))
		if status.State == "failed" || status.State == "canceled" {
			t.Fatalf("benchmark interrupted: %+v", status)
		}
	}
}

type measurementLogger struct {
	capturedLogger
	mu    sync.Mutex
	t     *testing.T
	phase string
	start time.Time
}

func (l *measurementLogger) Info(message string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if message == "movie scan phase" || message == "movie scan finished" {
		if !l.start.IsZero() {
			l.t.Logf("phase %s: %s", l.phase, time.Since(l.start))
		}
		l.start = time.Now()
		for i := 0; i+1 < len(args); i += 2 {
			if args[i] == "phase" {
				l.phase, _ = args[i+1].(string)
			}
		}
	}
	l.t.Log(message, args)
}
func processMeasurement(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unavailable on this platform"
	}
	if strings.HasSuffix(path, "/status") {
		lines := make([]string, 0)
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VmRSS:") || strings.HasPrefix(line, "VmHWM:") {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "; ")
	}
	return strings.ReplaceAll(strings.TrimSpace(string(data)), "\n", "; ")
}

func (l *measurementLogger) Warn(message string, args ...any)  { l.t.Log(message, args) }
func (l *measurementLogger) Error(message string, args ...any) { l.t.Log(message, args) }
