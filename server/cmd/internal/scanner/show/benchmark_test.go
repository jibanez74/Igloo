//go:build externalbin

package show

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"testing"

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
	probe := scannertest.RealProbe(t)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	log := scannertest.NewMeasurementLogger(t)
	s := New(Dependencies{DB: db, Queries: queries, Ffprobe: probe, Logger: log, ScanContext: ctx})
	for _, run := range []string{"fresh", "unchanged"} {
		scannertest.MeasureRun(t, run, func() string {
			err := s.scan(root)
			if err != nil {
				t.Fatal(err)
			}
			status := s.Status()
			return fmt.Sprintf("total=%d processed=%d imported=%d updated=%d unchanged=%d failed=%d deferred=%d episodes=%d", status.Total, status.Processed, status.Imported, status.Updated, status.Unchanged, status.Failed, status.Deferred, status.Episodes)
		})
	}
}
