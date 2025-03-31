package ffmpeg

import (
	"fmt"
	"os/exec"
	"time"
)

type job struct {
	process   *exec.Cmd
	startTime time.Time
	pid       string
	opts      *HlsOpts
	cleanup   func()
}

type JobInfo struct {
	PID       string
	StartTime time.Time
	Duration  time.Duration
}

func (f *ffmpeg) GetJobInfo(pid string) (*JobInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	job, exists := f.jobs[pid]
	if !exists {
		return nil, &ffmpegError{
			Field: "pid",
			Value: pid,
			Msg:   "job not found",
		}
	}

	return &JobInfo{
		PID:       job.pid,
		StartTime: job.startTime,
		Duration:  time.Since(job.startTime),
	}, nil
}

func (f *ffmpeg) CancelJob(pid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	job, exists := f.jobs[pid]
	if !exists {
		return &ffmpegError{
			Field: "pid",
			Value: pid,
			Msg:   "job not found",
		}
	}

	err := job.process.Process.Kill()
	if err != nil {
		return &ffmpegError{
			Field: "process",
			Value: pid,
			Msg:   fmt.Sprintf("failed to kill process: %v", err),
		}
	}

	delete(f.jobs, pid)
	return nil
}

func (f *ffmpeg) JobsFull(limit int) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if limit <= 0 {
		return false
	}

	return len(f.jobs) >= limit
}
