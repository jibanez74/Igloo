package ffprobe

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKeyframePacket(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantPts float64
		wantKey bool
		wantOK  bool
	}{
		{name: "keyframe row", line: "12.500000,K__", wantPts: 12.5, wantKey: true, wantOK: true},
		{name: "non-keyframe row", line: "13.250000,___", wantPts: 13.25, wantKey: false, wantOK: true},
		{name: "keyframe with discard flag", line: "0.000000,K_D", wantPts: 0, wantKey: true, wantOK: true},
		{name: "unset pts", line: "N/A,K__", wantOK: false},
		{name: "empty line", line: "", wantOK: false},
		{name: "single field", line: "12.500000", wantOK: false},
		{name: "non-numeric pts", line: "garbage,K__", wantOK: false},
		{name: "surrounding whitespace", line: "  7.000000,K__  \r", wantPts: 7, wantKey: true, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pts, isKeyframe, ok := parseKeyframePacket(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if pts != tt.wantPts {
				t.Errorf("pts = %v, want %v", pts, tt.wantPts)
			}
			if isKeyframe != tt.wantKey {
				t.Errorf("isKeyframe = %v, want %v", isKeyframe, tt.wantKey)
			}
		})
	}
}

func TestKeyframeAtOrBeforeRejectsInvalidInput(t *testing.T) {
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{})}

	tests := []struct {
		name      string
		filePath  string
		targetSec float64
		wantErr   string
	}{
		{name: "empty path", filePath: "", targetSec: 10, wantErr: "source path is required"},
		{name: "whitespace path", filePath: "   ", targetSec: 10, wantErr: "source path is required"},
		{name: "zero target", filePath: "/tmp/movie.mkv", targetSec: 0, wantErr: "target must be greater than zero"},
		{name: "negative target", filePath: "/tmp/movie.mkv", targetSec: -1, wantErr: "target must be greater than zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := probe.KeyframeAtOrBefore(context.Background(), tt.filePath, 0, tt.targetSec)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestKeyframeAtOrBeforeNoKeyframeFound(t *testing.T) {
	// A non-keyframe row before the target and a keyframe past it: neither may
	// satisfy the lookup.
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout: "599.000000,___\n601.500000,K__",
	})}

	_, err := probe.KeyframeAtOrBefore(context.Background(), "/tmp/movie.mkv", 0, 600)
	if err == nil || !strings.Contains(err.Error(), "no keyframe found at or before 600.000") {
		t.Fatalf("error = %v, want no keyframe found error", err)
	}
}

func TestKeyframeAtOrBeforeReturnsContextError(t *testing.T) {
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout: "599.000000,K__",
	})}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := probe.KeyframeAtOrBefore(ctx, "/tmp/movie.mkv", 0, 600)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestKeyframeAtOrBeforeUsesAbsoluteBoundedInterval(t *testing.T) {
	argsLog := filepath.Join(t.TempDir(), "args.log")
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout:  "565.000000,K__\n599.000000,K__\n600.500000,K__",
		argsLog: argsLog,
	})}

	keyframe, err := probe.KeyframeAtOrBefore(
		context.Background(),
		"/tmp/movie.mkv",
		2,
		600,
	)
	if err != nil {
		t.Fatalf("KeyframeAtOrBefore failed: %v", err)
	}
	if keyframe != 599 {
		t.Fatalf("keyframe = %.3f, want 599", keyframe)
	}

	requireArgumentValue(t, readArgumentLog(t, argsLog), "-read_intervals", "570.000%601.000")
}

func TestKeyframeAtOrBeforeClampsNearZeroInterval(t *testing.T) {
	argsLog := filepath.Join(t.TempDir(), "args.log")
	probe := &ffprobe{bin: writeFakeFFprobe(t, fakeFFprobeSpec{
		stdout:  "0.000000,K__",
		argsLog: argsLog,
	})}

	keyframe, err := probe.KeyframeAtOrBefore(
		context.Background(),
		"/tmp/movie.mkv",
		0,
		0.25,
	)
	if err != nil {
		t.Fatalf("KeyframeAtOrBefore failed: %v", err)
	}
	if keyframe != 0 {
		t.Fatalf("keyframe = %.3f, want 0", keyframe)
	}

	requireArgumentValue(t, readArgumentLog(t, argsLog), "-read_intervals", "0.000%1.250")
}
