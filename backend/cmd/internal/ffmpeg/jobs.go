package ffmpeg

import (
	"fmt"
	"os/exec"
	"time"
)

type job struct {
	process   *exec.Cmd
	startTime time.Time
	status    string
}

func (f *ffmpeg) CancelJob(pid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, exists := f.jobs[pid]
	if !exists {
		return fmt.Errorf("job %s not found", pid)
	}

	err := job.process.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	delete(f.jobs, pid)

	return nil
}

func (f *ffmpeg) JobsFull(limit int) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return len(f.jobs) >= limit
}
