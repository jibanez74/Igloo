//go:build externalbin

package show

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
// Set IGLOO_BENCH_SHOWS_DIR and IGLOO_FFPROBE_PATH to run against a real library.
func TestShowLibraryBenchmark(t *testing.T) {
	root := os.Getenv("IGLOO_BENCH_SHOWS_DIR")
	if root == "" {
		t.Skip("set IGLOO_BENCH_SHOWS_DIR for read-only real-library measurements")
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
	for _, run := range []string{"fresh", "unchanged"} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		t.Logf("%s initial I/O: %s", run, scannertest.ProcessMeasurement("/proc/self/io"))
		err := s.scan(root)
		if err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		status := s.Status()
		t.Logf("%s: elapsed=%s total=%d processed=%d imported=%d updated=%d unchanged=%d failed=%d deferred=%d episodes=%d heap=%d allocated=%d", run, time.Since(start), status.Total, status.Processed, status.Imported, status.Updated, status.Unchanged, status.Failed, status.Deferred, status.Episodes, after.HeapAlloc, after.TotalAlloc-before.TotalAlloc)
		t.Logf("%s final I/O: %s; memory: %s", run, scannertest.ProcessMeasurement("/proc/self/io"), scannertest.ProcessMeasurement("/proc/self/status"))
	}
}
