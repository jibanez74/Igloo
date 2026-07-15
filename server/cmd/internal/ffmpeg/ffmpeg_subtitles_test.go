package ffmpeg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestExtractSubtitleAsWebVTTMapsAbsoluteStreamAndNormalizesHardSpaces(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "arguments.log")
	body := "printf '%s\\n' \"$@\" > " + formatShellPath(logPath) + `
printf '%s\n' 'WEBVTT' '' '00:00.000 --> 00:01.000' 'hello\hworld'
`
	script := writeFakeFFmpeg(t, "fake ffmpeg", body)
	f := &ffmpeg{bin: script}
	sourcePath := filepath.Join(t.TempDir(), "movie source.mkv")

	output, err := f.ExtractSubtitleAsWebVTT(context.Background(), sourcePath, 7)
	if err != nil {
		t.Fatalf("ExtractSubtitleAsWebVTT: %v", err)
	}
	if !strings.HasPrefix(string(output), "WEBVTT") {
		t.Fatalf("output = %q, want WebVTT", output)
	}
	if !strings.Contains(string(output), "hello world") || strings.Contains(string(output), `\h`) {
		t.Fatalf("hard-space normalization failed: %q", output)
	}

	argumentData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read argument log: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argumentData)), "\n")
	requireArgumentValue(t, args, "-i", sourcePath)
	requireArgumentValue(t, args, "-map", "0:7")
	requireArgumentValue(t, args, "-c:s", "webvtt")
	requireArgumentValue(t, args, "-f", "webvtt")
	if args[len(args)-1] != "pipe:1" {
		t.Fatalf("last argument = %q, want pipe:1", args[len(args)-1])
	}
}

func TestExtractSubtitleAsWebVTTReportsNonzeroExit(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantDiagnostic bool
	}{
		{name: "with stderr", body: "printf '%s\\n' conversion-failed >&2\nexit 4\n", wantDiagnostic: true},
		{name: "without stderr", body: "exit 5\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := writeFakeFFmpeg(t, "fake ffmpeg", tt.body)
			f := &ffmpeg{bin: script}

			_, err := f.ExtractSubtitleAsWebVTT(context.Background(), "/movie.mkv", 2)
			if err == nil {
				t.Fatal("expected subtitle extraction error")
			}
			if !strings.Contains(err.Error(), "ffmpeg subtitle extraction failed") {
				t.Fatalf("error = %q, want extraction context", err.Error())
			}
			hasDiagnostic := strings.Contains(err.Error(), "conversion-failed")
			if hasDiagnostic != tt.wantDiagnostic {
				t.Fatalf("diagnostic presence = %v, want %v: %q", hasDiagnostic, tt.wantDiagnostic, err.Error())
			}
		})
	}
}

func TestExtractSubtitleAsWebVTTBoundsDiagnosticTailOnUTF8Boundary(t *testing.T) {
	diagnostic := "discard-this-prefix-" + strings.Repeat("é", 3000) + "-diagnostic-tail"
	body := "printf '%s' '" + diagnostic + "' >&2\nexit 6\n"
	script := writeFakeFFmpeg(t, "fake ffmpeg", body)
	f := &ffmpeg{bin: script}

	_, err := f.ExtractSubtitleAsWebVTT(context.Background(), "/movie.mkv", 1)
	if err == nil {
		t.Fatal("expected subtitle extraction error")
	}
	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("diagnostic is not valid UTF-8: %q", message)
	}
	if !strings.Contains(message, "diagnostic-tail") {
		t.Fatalf("diagnostic tail was lost: %q", message)
	}
	if strings.Contains(message, "discard-this-prefix") {
		t.Fatalf("diagnostic prefix was not truncated: %d bytes", len(message))
	}
	if len(message) > 4300 {
		t.Fatalf("bounded diagnostic error is too large: %d bytes", len(message))
	}
}

func TestExtractSubtitleAsWebVTTCancellationTerminatesProcess(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "started")
	body := "printf '%s\\n' started > " + formatShellPath(markerPath) + "\nexec sleep 30\n"
	script := writeFakeFFmpeg(t, "fake ffmpeg", body)
	f := &ffmpeg{bin: script}
	ctx, cancel := context.WithCancel(context.Background())
	type subtitleResult struct {
		output []byte
		err    error
	}
	results := make(chan subtitleResult, 1)

	go func() {
		output, err := f.ExtractSubtitleAsWebVTT(ctx, "/movie with spaces.mkv", 3)
		results <- subtitleResult{output: output, err: err}
	}()

	deadline := time.Now().Add(testProcessTimeout)
	for {
		_, err := os.Stat(markerPath)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("timed out waiting for subtitle process to start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case result := <-results:
		if result.output != nil {
			t.Fatalf("canceled output = %q, want nil", result.output)
		}
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("error = %v, want context cancellation", result.err)
		}
	case <-time.After(testProcessTimeout):
		t.Fatal("subtitle process did not terminate after cancellation")
	}
}
