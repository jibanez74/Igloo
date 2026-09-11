package scanner

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRunWorkersDeliversEveryResultOnTheCaller(t *testing.T) {
	jobs := make([]int, 50)
	for i := range jobs {
		jobs[i] = i
	}
	var inFlight, peak atomic.Int32
	var mu sync.Mutex
	seen := make(map[int]bool)
	dispatched := 0
	RunWorkers(context.Background(), 3, jobs, func() bool { return true },
		func(_ context.Context, job int) int {
			current := inFlight.Add(1)
			for {
				previous := peak.Load()
				if current <= previous || peak.CompareAndSwap(previous, current) {
					break
				}
			}
			inFlight.Add(-1)
			return job * 2
		},
		func(int) {
			mu.Lock()
			dispatched++
			mu.Unlock()
		},
		func(result int) {
			mu.Lock()
			seen[result/2] = true
			mu.Unlock()
		})
	if dispatched != len(jobs) || len(seen) != len(jobs) || peak.Load() > 3 {
		t.Fatalf("dispatched=%d results=%d peak=%d", dispatched, len(seen), peak.Load())
	}
}

func TestRunWorkersDropsRemainingJobsWhenStoppedOrCanceled(t *testing.T) {
	jobs := []int{1, 2, 3, 4, 5}
	stopped := false
	dispatched, results := 0, 0
	RunWorkers(context.Background(), 2, jobs, func() bool { return !stopped },
		func(_ context.Context, job int) int { return job },
		func(int) { dispatched++ },
		func(int) {
			results++
			stopped = true
		})
	// The gate is a stop, not a pause: once the first result closes it the
	// remaining jobs are dropped, only in-flight ones finish, and every
	// dispatched job still reports back.
	if dispatched >= len(jobs) || results != dispatched {
		t.Fatalf("stopped dispatch: dispatched=%d results=%d", dispatched, results)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results = 0
	RunWorkers(ctx, 2, jobs, func() bool { return true },
		func(_ context.Context, job int) int { return job },
		func(int) { t.Fatal("dispatched after cancellation") },
		func(int) { results++ })
	if results != 0 {
		t.Fatalf("canceled context delivered %d results", results)
	}
}
