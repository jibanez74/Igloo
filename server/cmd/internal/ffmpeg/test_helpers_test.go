package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

const testProcessTimeout = 10 * time.Second

type hlsExitResult struct {
	exitErr    error
	stderrTail []string
}

func writeFakeFFmpeg(t *testing.T, name string, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	contents := "#!/bin/sh\nset -eu\n" + body
	if !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	err := os.WriteFile(path, []byte(contents), 0755)
	if err != nil {
		t.Fatalf("write fake FFmpeg: %v", err)
	}
	return path
}

func waitForHLSExit(t *testing.T, results <-chan hlsExitResult) hlsExitResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testProcessTimeout)
	defer cancel()
	select {
	case result := <-results:
		return result
	case <-ctx.Done():
		t.Fatal("timed out waiting for FFmpeg exit callback")
		return hlsExitResult{}
	}
}

func waitForCommandExit(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	deadline := time.Now().Add(testProcessTimeout)
	for time.Now().Before(deadline) {
		err := cmd.Process.Signal(syscall.Signal(0))
		if errors.Is(err, os.ErrProcessDone) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for FFmpeg process to exit")
}

func requireArgumentValue(t *testing.T, args []string, flag string, want string) {
	t.Helper()
	index := indexOf(args, flag)
	if index < 0 {
		t.Fatalf("missing argument %q in %v", flag, args)
	}
	if index+1 >= len(args) {
		t.Fatalf("argument %q has no value in %v", flag, args)
	}
	if args[index+1] != want {
		t.Fatalf("%s = %q, want %q", flag, args[index+1], want)
	}
}

func requireArgumentBefore(t *testing.T, args []string, first string, second string) {
	t.Helper()
	firstIndex := indexOf(args, first)
	secondIndex := indexOf(args, second)
	if firstIndex < 0 || secondIndex < 0 {
		t.Fatalf("expected %q and %q in %v", first, second, args)
	}
	if firstIndex >= secondIndex {
		t.Fatalf("argument %q at %d must precede %q at %d", first, firstIndex, second, secondIndex)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsMatching(values []string, match func(string) bool) bool {
	for _, value := range values {
		if match(value) {
			return true
		}
	}
	return false
}

func indexOf(args []string, flag string) int {
	for i, arg := range args {
		if arg == flag {
			return i
		}
	}
	return -1
}

func basicHLSParams(outDir string) HLSParams {
	return HLSParams{
		SourcePath:       "/tmp/source file.mkv",
		OutDir:           outDir,
		Profile:          helpers.HLS_PROFILE_720P_3MBPS,
		VideoStreamIndex: 0,
		AudioStreamIndex: 1,
		HWDevice:         "cpu",
		Capabilities:     Capabilities{Probed: true},
	}
}

func formatShellPath(path string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(path, "'", "'\\''"))
}
