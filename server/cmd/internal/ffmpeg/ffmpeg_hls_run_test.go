package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

func TestRunHLSRejectsInvalidOutputPaths(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", "exit 0\n")
	f := &ffmpeg{bin: script}
	filePath := filepath.Join(t.TempDir(), "output-file")
	err := os.WriteFile(filePath, []byte("not a directory"), 0644)
	if err != nil {
		t.Fatalf("write output file: %v", err)
	}

	tests := []struct {
		name    string
		outDir  string
		wantErr string
	}{
		{name: "empty", outDir: "   ", wantErr: "output directory is required"},
		{name: "missing", outDir: filepath.Join(t.TempDir(), "missing"), wantErr: "stat output directory"},
		{name: "file", outDir: filePath, wantErr: "not a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, runErr := f.RunHLS(context.Background(), basicHLSParams(tt.outDir), nil)
			if runErr == nil {
				t.Fatal("expected output path error")
			}
			if !strings.Contains(runErr.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", runErr.Error(), tt.wantErr)
			}
		})
	}
}

func TestRunHLSStartFailureDoesNotDeliverCallback(t *testing.T) {
	results := make(chan hlsExitResult, 1)
	f := &ffmpeg{bin: filepath.Join(t.TempDir(), "missing-ffmpeg")}

	_, err := f.RunHLS(context.Background(), basicHLSParams(t.TempDir()), func(exitErr error, stderrTail []string) {
		results <- hlsExitResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err == nil {
		t.Fatal("expected process start failure")
	}

	select {
	case result := <-results:
		t.Fatalf("callback delivered after start failure: %#v", result)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestRunHLSDeliversExactlyOneCallbackForSuccessAndFailure(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantFailure bool
	}{
		{name: "success", body: "printf '%s\\n' success >&2\nexit 0\n"},
		{name: "failure", body: "printf '%s\\n' failure >&2\nexit 7\n", wantFailure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := writeFakeFFmpeg(t, "fake ffmpeg", tt.body)
			f := &ffmpeg{bin: script}
			results := startFakeHLS(t, f, basicHLSParams(t.TempDir()))

			result := waitForHLSExit(t, results)
			gotFailure := result.exitErr != nil
			if gotFailure != tt.wantFailure {
				t.Fatalf("exitErr = %v, want failure %v", result.exitErr, tt.wantFailure)
			}
			if len(result.stderrTail) != 1 || result.stderrTail[0] != tt.name {
				t.Fatalf("stderr tail = %v, want [%s]", result.stderrTail, tt.name)
			}

			select {
			case duplicate := <-results:
				t.Fatalf("duplicate callback: %#v", duplicate)
			case <-time.After(150 * time.Millisecond):
			}
		})
	}
}

func TestRunHLSCancellationTerminatesProcess(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", "printf '%s\\n' started >&2\nexec sleep 30\n")
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan hlsExitResult, 1)
	f := &ffmpeg{bin: script}

	cmd, err := f.RunHLS(ctx, basicHLSParams(t.TempDir()), func(exitErr error, stderrTail []string) {
		results <- hlsExitResult{exitErr: exitErr, stderrTail: stderrTail}
	})
	if err != nil {
		cancel()
		t.Fatalf("RunHLS: %v", err)
	}
	cancel()

	result := waitForHLSExit(t, results)
	if result.exitErr == nil {
		t.Fatal("canceled FFmpeg exited successfully")
	}
	if cmd.ProcessState == nil {
		t.Fatal("canceled FFmpeg process was not reaped before callback")
	}
}

func TestRunHLSAllowsNilCallback(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", "exit 0\n")
	f := &ffmpeg{bin: script}

	cmd, err := f.RunHLS(context.Background(), basicHLSParams(t.TempDir()), nil)
	if err != nil {
		t.Fatalf("RunHLS: %v", err)
	}
	waitForCommandExit(t, cmd)
}

func TestRunHLSRetainsOrderedLastTwentyStderrLines(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", `
i=1
while [ "$i" -le 25 ]; do
  printf 'line-%02d\n' "$i" >&2
  i=$((i + 1))
done
exit 3
`)
	f := &ffmpeg{bin: script}
	result := runFakeHLS(t, f, basicHLSParams(t.TempDir()))

	if result.exitErr == nil {
		t.Fatal("expected nonzero exit")
	}
	if len(result.stderrTail) != hlsStderrTailLines {
		t.Fatalf("stderr tail length = %d, want %d", len(result.stderrTail), hlsStderrTailLines)
	}
	for i, line := range result.stderrTail {
		want := fmt.Sprintf("line-%02d", i+6)
		if line != want {
			t.Fatalf("stderr tail line %d = %q, want %q; tail=%v", i, line, want, result.stderrTail)
		}
	}
}

func TestRunHLSCapturesLongStderrLine(t *testing.T) {
	longLine := strings.Repeat("x", 128*1024)
	body := fmt.Sprintf("printf '%%s\\n' '%s' >&2\n", longLine)
	script := writeFakeFFmpeg(t, "fake ffmpeg", body)
	f := &ffmpeg{bin: script}

	result := runFakeHLS(t, f, basicHLSParams(t.TempDir()))
	if result.exitErr != nil {
		t.Fatalf("exitErr = %v, want nil", result.exitErr)
	}
	if len(result.stderrTail) != 1 {
		t.Fatalf("captured stderr line count = %d, want 1", len(result.stderrTail))
	}
	if len(result.stderrTail[0]) != len(longLine) {
		t.Fatalf("captured stderr line length = %d, want %d", len(result.stderrTail[0]), len(longLine))
	}
}

func TestRunHLSReportsStderrScannerErrors(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", "head -c 2000000 /dev/zero | tr '\\000' x >&2\nexit 1\n")
	f := &ffmpeg{bin: script}

	result := runFakeHLS(t, f, basicHLSParams(t.TempDir()))
	if result.exitErr == nil {
		t.Fatal("expected nonzero exit")
	}
	hasScannerError := slices.ContainsFunc(result.stderrTail, func(line string) bool {
		return strings.Contains(line, "stderr scan error:") && strings.Contains(line, "token too long")
	})
	if !hasScannerError {
		t.Fatalf("stderr tail missing scanner error: %v", result.stderrTail)
	}
}

func TestRunHLSConcurrentSessionsKeepStateIsolated(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", `
tag=missing
previous=
for argument in "$@"; do
  if [ "$previous" = "-i" ]; then
    tag=$argument
    break
  fi
  previous=$argument
done
printf '%s-first\n' "$tag" >&2
printf '%s-last\n' "$tag" >&2
exit 0
`)
	f := &ffmpeg{bin: script}
	alpha := basicHLSParams(t.TempDir())
	alpha.SourcePath = "alpha source"
	beta := basicHLSParams(t.TempDir())
	beta.SourcePath = "beta source"

	alphaResults := startFakeHLS(t, f, alpha)
	betaResults := startFakeHLS(t, f, beta)

	alphaResult := waitForHLSExit(t, alphaResults)
	betaResult := waitForHLSExit(t, betaResults)
	if alphaResult.exitErr != nil || betaResult.exitErr != nil {
		t.Fatalf("concurrent exits: alpha=%v beta=%v", alphaResult.exitErr, betaResult.exitErr)
	}
	if strings.Join(alphaResult.stderrTail, " ") != "alpha source-first alpha source-last" {
		t.Fatalf("alpha stderr was not isolated: %v", alphaResult.stderrTail)
	}
	if strings.Join(betaResult.stderrTail, " ") != "beta source-first beta source-last" {
		t.Fatalf("beta stderr was not isolated: %v", betaResult.stderrTail)
	}
}

func TestRunHLSUsesAbsoluteOutputDirectoryAndInternalCapabilities(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	parent := t.TempDir()
	outDir := filepath.Join(parent, "output with spaces")
	err = os.Mkdir(outDir, 0755)
	if err != nil {
		t.Fatalf("mkdir output: %v", err)
	}
	relativeOutDir, err := filepath.Rel(cwd, outDir)
	if err != nil {
		t.Fatalf("relative output path: %v", err)
	}

	// The fake writes both files relative to its working directory, which is
	// how the test confirms RunHLS resolved and entered the absolute out dir.
	script := writeFakeFFmpeg(t, "fake ffmpeg", "printf '%s\\n' \"$PWD\" > pwd.txt\nprintf '%s\\n' \"$@\" > args.txt\n")
	f := &ffmpeg{
		bin:          script,
		capabilities: hlsTestCapabilitiesForDevice(helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA),
	}
	params := basicHLSParams(relativeOutDir)
	params.HWDevice = helpers.HARDWARE_ACCELERATION_DEVICE_NVIDIA
	params.Capabilities = Capabilities{}

	result := runFakeHLS(t, f, params)
	if result.exitErr != nil {
		t.Fatalf("exitErr = %v", result.exitErr)
	}

	pwdData, err := os.ReadFile(filepath.Join(outDir, "pwd.txt"))
	if err != nil {
		t.Fatalf("read pwd: %v", err)
	}
	if strings.TrimSpace(string(pwdData)) != outDir {
		t.Fatalf("command directory = %q, want %q", strings.TrimSpace(string(pwdData)), outDir)
	}

	args := readArgumentLog(t, filepath.Join(outDir, "args.txt"))
	if !slices.Contains(args, "h264_nvenc") || !slices.Contains(args, "cuda") {
		t.Fatalf("internal capabilities were not used: %v", args)
	}
	requireArgumentValue(t, args, "-hls_segment_filename", filepath.Join(outDir, "segment_%d.m4s"))
	if args[len(args)-1] != filepath.Join(outDir, helpers.HLS_PLAYLIST_FILENAME) {
		t.Fatalf("playlist path = %q", args[len(args)-1])
	}
}
