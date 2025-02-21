package ffmpeg

import (
	"errors"
	"fmt"
	"os/exec"
	"time"
)

type job struct {
	id        string
	process   *exec.Cmd
	startTime time.Time
	status    string
	error     error
}

func (f *ffmpeg) CancelJob(pid string) error {
	job, ok := f.jobs[pid]
	if !ok {
		return errors.New("job not found")
	}

	err := job.process.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill job: %w", err)
	}

	delete(f.jobs, pid)

	return nil
}
