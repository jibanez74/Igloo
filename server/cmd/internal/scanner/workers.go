package scanner

import (
	"context"
	"sync"
)

// RunWorkers processes jobs on a fixed pool while the calling goroutine stays
// the sole owner of scan state: onDispatch and onResult run on the caller, so
// they may touch report and index maps without locking. Dispatch pauses when
// canDispatch returns false or ctx is canceled; results already in flight are
// still delivered. It is shared by the movie probe and enrichment phases.
func RunWorkers[J, R any](
	ctx context.Context,
	workers int,
	jobs []J,
	canDispatch func() bool,
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
		ready := next < len(jobs) && active < workers && ctx.Err() == nil && canDispatch()
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
