//go:build externalbin

package movie

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner/scannertest"
)

// Opt-in: media is opened read-only; the database always lives in t.TempDir.
// Set IGLOO_BENCH_MOVIES_DIR and IGLOO_FFPROBE_PATH to run against a real library.
func TestMovieLibraryBenchmark(t *testing.T) {
	root := os.Getenv("IGLOO_BENCH_MOVIES_DIR")
	if root == "" {
		t.Skip("set IGLOO_BENCH_MOVIES_DIR for read-only real-library measurements")
	}
	db, queries := scannertest.OpenDB(t, filepath.Join(t.TempDir(), "benchmark.db")+"?_foreign_keys=on")
	probe, err := ffprobe.New()
	if err != nil {
		t.Fatal(err)
	}
	defer ffprobe.Cleanup()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	log := scannertest.NewMeasurementLogger(t)
	s := New(Dependencies{DB: db, Queries: queries, Ffprobe: probe, Logger: log, ScanContext: ctx})
	for _, run := range []string{"fresh", "recovery", "unchanged"} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		t.Logf("%s initial I/O: %s", run, scannertest.ProcessMeasurement("/proc/self/io"))
		s.scan(root)
		runtime.ReadMemStats(&after)
		status := s.Status()
		t.Logf("%s: elapsed=%s total=%d processed=%d imported=%d unchanged=%d failed=%d deferred=%d heap=%d allocated=%d", run, time.Since(start), status.Total, status.Processed, status.Imported, status.Unchanged, status.Failed, status.Deferred, after.HeapAlloc, after.TotalAlloc-before.TotalAlloc)
		t.Logf("%s final I/O: %s; memory: %s", run, scannertest.ProcessMeasurement("/proc/self/io"), scannertest.ProcessMeasurement("/proc/self/status"))
		if status.State == "failed" || status.State == "canceled" {
			t.Fatalf("benchmark interrupted: %+v", status)
		}
	}
}
