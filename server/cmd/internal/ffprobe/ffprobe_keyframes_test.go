package ffprobe

import (
	"context"
	"os"
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

func TestKeyframeAtOrBeforeUsesAbsoluteBoundedInterval(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.log")
	t.Setenv("FFPROBE_ARGS_LOG", argsPath)

	binPath := filepath.Join(t.TempDir(), "ffprobe")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FFPROBE_ARGS_LOG"
printf '%s\n' '565.000000,K__'
printf '%s\n' '599.000000,K__'
printf '%s\n' '600.500000,K__'
`
	err := os.WriteFile(binPath, []byte(script), 0755)
	if err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	probe := &ffprobe{bin: binPath}
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

	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read ffprobe arguments: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	for index, arg := range args {
		if arg != "-read_intervals" {
			continue
		}
		if index+1 >= len(args) {
			t.Fatal("-read_intervals has no value")
		}
		if args[index+1] != "570.000%601.000" {
			t.Fatalf("read interval = %q, want %q", args[index+1], "570.000%601.000")
		}
		return
	}

	t.Fatal("-read_intervals argument not found")
}

func TestKeyframeAtOrBeforeClampsNearZeroInterval(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.log")
	t.Setenv("FFPROBE_ARGS_LOG", argsPath)

	binPath := filepath.Join(t.TempDir(), "ffprobe")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FFPROBE_ARGS_LOG"
printf '%s\n' '0.000000,K__'
`
	err := os.WriteFile(binPath, []byte(script), 0755)
	if err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	probe := &ffprobe{bin: binPath}
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

	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read ffprobe arguments: %v", err)
	}
	if !strings.Contains(string(argsData), "0.000%1.250") {
		t.Fatalf("arguments do not contain clamped interval: %q", argsData)
	}
}
