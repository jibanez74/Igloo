package scanner

import (
	"context"
	"sync"
)

// RunWorkers processes jobs on a fixed pool while the calling goroutine stays
// the sole owner of scan state: onDispatch and onResult run on the caller, so
// they may touch report and index maps without locking.
//
// shouldContinue is a stop gate, not a pause: once it returns false (or ctx is
// canceled) no further job is dispatched, and the remaining jobs are dropped as
// soon as the in-flight ones drain -- the loop does not wait for it to reopen.
// Results already in flight are always delivered. Both callers want that: the
// movie retry window closes for good, and so does the TMDB circuit breaker.
func RunWorkers[J, R any](
	ctx context.Context,
	workers int,
	jobs []J,
	shouldContinue func() bool,
	run func(context.Context, J) R,
	onDispatch func(J),
	onResult func(R),
) {
	queue := make(chan J)
	results := make(chan R, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for job := range queue {
				results <- run(ctx, job)
			}
		}()
	}
	defer group.Wait()
	next, active := 0, 0
	for next < len(jobs) || active > 0 {
		var dispatch chan J
		var job J
		ready := next < len(jobs) && active < workers && ctx.Err() == nil && shouldContinue()
		if ready {
			job = jobs[next]
			dispatch = queue
		}
		idle := active == 0 && dispatch == nil
		if idle {
			break
		}
		select {
		case dispatch <- job:
			next++
			active++
			onDispatch(job)
		case result := <-results:
			active--
			onResult(result)
		}
	}
	close(queue)
}
