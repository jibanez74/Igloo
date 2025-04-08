package ffmpeg

import (
	"context"
	"fmt"
)

type job struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// CancelJob cancels a running job by its ID
func (f *ffmpeg) CancelJob(jobID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, exists := f.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	job.cancel()
	delete(f.jobs, jobID)

	return nil
}

func (f *ffmpeg) JobsFull() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return len(f.jobs) >= f.maxJobs
}
