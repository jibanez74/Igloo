package fakebin

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestWriteSpecRunsAndLogs(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "it's logs")
	err := os.Mkdir(logDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	argsLog := filepath.Join(logDir, "args.log")
	callLog := filepath.Join(logDir, "calls.log")
	bin := WriteSpec(t, "fake tool", Spec{
		Stdout:   "out 'quoted'",
		Stderr:   "err",
		ExitCode: 3,
		ArgsLog:  argsLog,
		CallLog:  callLog,
	})

	for _, args := range [][]string{{"-version"}, {"-i", "a b.mkv"}, {"-version"}} {
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(bin, args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exitErr *exec.ExitError
		isExit3 := errors.As(err, &exitErr) && exitErr.ExitCode() == 3
		if !isExit3 {
			t.Fatalf("run %v: err = %v, want exit 3", args, err)
		}
		gotStdout, gotStderr := stdout.String(), stderr.String()
		if gotStdout != "out 'quoted'\n" || gotStderr != "err\n" {
			t.Fatalf("run %v: stdout %q, stderr %q", args, gotStdout, gotStderr)
		}
	}

	gotArgs, wantArgs := ReadLog(t, argsLog), []string{"-version"}
	if !slices.Equal(gotArgs, wantArgs) {
		t.Fatalf("args log = %q, want %q (overwritten by the last call)", gotArgs, wantArgs)
	}
	gotCalls, wantCalls := ReadLog(t, callLog), []string{"-version", "-i a b.mkv", "-version"}
	if !slices.Equal(gotCalls, wantCalls) {
		t.Fatalf("call log = %q, want %q", gotCalls, wantCalls)
	}
	versionCalls := CountLine(t, callLog, "-version")
	if versionCalls != 2 {
		t.Fatalf("CountLine = %d, want 2", versionCalls)
	}
}

func TestWriteHeaderStopsOnErrorsAndUnsetVariables(t *testing.T) {
	for name, body := range map[string]string{
		"set -e": "false\necho unreachable",
		"set -u": "echo \"$IGLOO_FAKEBIN_UNSET\"",
	} {
		t.Run(name, func(t *testing.T) {
			bin := Write(t, "fake", body)
			out, err := exec.Command(bin).Output()
			if err == nil || len(out) != 0 {
				t.Fatalf("output %q, err %v; want the header to stop the script", out, err)
			}
		})
	}
}
