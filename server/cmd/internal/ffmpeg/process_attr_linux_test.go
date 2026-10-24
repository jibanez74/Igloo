package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const pdeathsigHelperEnv = "IGLOO_TEST_PDEATHSIG_HELPER_DIR"

// processStat reads a process's state and process group from /proc. The
// command name sits in parentheses and may contain spaces, so fields are
// counted from the closing one.
func processStat(pid int) (state string, pgrp int, err error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", 0, err
	}
	stat := string(raw)
	fields := strings.Fields(stat[strings.LastIndex(stat, ")")+1:])
	if len(fields) < 3 {
		return "", 0, fmt.Errorf("short /proc/%d/stat: %q", pid, stat)
	}
	pgrp, err = strconv.Atoi(fields[2])
	return fields[0], pgrp, err
}

// processGone reports a process that no longer runs; a zombie waiting for
// its new parent to reap it counts as gone.
func processGone(pid int) bool {
	state, _, err := processStat(pid)
	return errors.Is(err, os.ErrNotExist) || state == "Z"
}

// A Ctrl-C goes to the terminal's whole foreground process group. FFmpeg in
// that group died with the server's own signal, before shutdown could mark
// its session stopped, and every session was logged as failed.
func TestRunHLSStartsFFmpegInItsOwnProcessGroup(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", "exec sleep 30\n")
	f := &ffmpeg{bin: script}

	ctx, cancel := context.WithCancel(context.Background())
	results, cmd := startHLS(t, ctx, f, basicHLSParams(t.TempDir()))
	defer func() {
		cancel()
		waitForHLSExit(t, results)
	}()

	_, pgrp, err := processStat(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("read FFmpeg process stat: %v", err)
	}
	if pgrp != cmd.Process.Pid {
		t.Fatalf("FFmpeg process group = %d, want its own group %d", pgrp, cmd.Process.Pid)
	}
	if pgrp == syscall.Getpgrp() {
		t.Fatal("FFmpeg shares the server's process group")
	}
}

// A server that dies without shutting down (a crash, SIGKILL, the OOM
// killer) used to leave its encodes running for the rest of the film. The
// helper below plays the server: it starts a session and exits on the spot.
func TestRunHLSStopsFFmpegWhenTheServerDies(t *testing.T) {
	dir := t.TempDir()
	helper := exec.Command(os.Args[0], "-test.run=^TestPdeathsigHelperProcess$", "-test.count=1")
	helper.Env = append(os.Environ(), pdeathsigHelperEnv+"="+dir)
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatalf("helper stdout: %v", err)
	}
	err = helper.Start()
	if err != nil {
		t.Fatalf("start helper: %v", err)
	}

	pid := 0
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		value, found := strings.CutPrefix(scanner.Text(), "ffmpeg-pid=")
		if found {
			pid, _ = strconv.Atoi(value)
		}
	}
	_ = helper.Wait()
	if pid <= 0 {
		t.Fatal("helper did not report the FFmpeg pid")
	}

	deadline := time.Now().Add(testProcessTimeout)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("FFmpeg pid %d outlived the server that started it", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestPdeathsigHelperProcess is the dying server, run only as a subprocess of
// TestRunHLSStopsFFmpegWhenTheServerDies.
func TestPdeathsigHelperProcess(t *testing.T) {
	dir := os.Getenv(pdeathsigHelperEnv)
	if dir == "" {
		t.Skip("helper process for TestRunHLSStopsFFmpegWhenTheServerDies")
	}

	script := filepath.Join(dir, "fake-ffmpeg")
	err := os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 60\n"), 0755)
	if err != nil {
		fmt.Println("write fake:", err)
		os.Exit(1)
	}
	f := &ffmpeg{bin: script}
	cmd, err := f.RunHLS(context.Background(), basicHLSParams(dir), nil)
	if err != nil {
		fmt.Println("RunHLS:", err)
		os.Exit(1)
	}
	fmt.Printf("ffmpeg-pid=%d\n", cmd.Process.Pid)
	// Exit without stopping the session, as a crash would.
	os.Exit(0)
}
