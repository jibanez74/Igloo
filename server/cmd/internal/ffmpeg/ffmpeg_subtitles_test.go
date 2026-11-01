package ffmpeg

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestExtractSubtitleAsWebVTTMapsAbsoluteStreamAndNormalizesHardSpaces(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "arguments.log")
	body := writeArgumentLog(logPath) + `
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

	args := readArgumentLog(t, logPath)
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

// The argument order is the behavior: an input -ss before -i only skips ahead,
// while the output -ss after the mapping drops and rebases packets, and the
// real-FFmpeg integration test pins what each combination does to timestamps.
func TestSubtitleStreamArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params SubtitleStreamParams
		want   []string
	}{
		{
			name:   "whole track copied from the source",
			params: SubtitleStreamParams{InputPath: "/media/movie.mkv", StreamIndex: 5},
			want: []string{
				"-v", "error", "-nostdin",
				"-i", "/media/movie.mkv", "-copyts", "-map", "0:5", "-c:s", "copy",
				"-map_chapters", "-1", "-map_metadata:g", "-1",
				"-flush_packets", "1", "-f", "matroska", "pipe:1",
			},
		},
		{
			name:   "copied from the source at a session start",
			params: SubtitleStreamParams{InputPath: "/media/movie.mkv", StreamIndex: 5, StartSec: 1800.25},
			want: []string{
				"-v", "error", "-nostdin", "-seek_timestamp", "1", "-ss", "1800.250",
				"-i", "/media/movie.mkv", "-copyts", "-map", "0:5", "-c:s", "copy",
				"-map_chapters", "-1", "-map_metadata:g", "-1", "-ss", "1800.250",
				"-flush_packets", "1", "-f", "matroska", "pipe:1",
			},
		},
		{
			name:   "converted from the source at a session start",
			params: SubtitleStreamParams{InputPath: "/media/movie.mp4", StreamIndex: 2, ConvertToSubRip: true, StartSec: 591.1744},
			want: []string{
				"-v", "error", "-nostdin", "-seek_timestamp", "1", "-ss", "591.174",
				"-i", "/media/movie.mp4", "-copyts", "-map", "0:2", "-c:s", "srt",
				"-map_chapters", "-1", "-map_metadata:g", "-1", "-ss", "591.174", "-output_ts_offset", "-591.174",
				"-flush_packets", "1", "-f", "matroska", "pipe:1",
			},
		},
		{
			name:   "converted from the source from the beginning",
			params: SubtitleStreamParams{InputPath: "/media/movie.mp4", StreamIndex: 2, ConvertToSubRip: true},
			want: []string{
				"-v", "error", "-nostdin",
				"-i", "/media/movie.mp4", "-copyts", "-map", "0:2", "-c:s", "srt",
				"-map_chapters", "-1", "-map_metadata:g", "-1",
				"-flush_packets", "1", "-f", "matroska", "pipe:1",
			},
		},
		{
			name:   "cached track is never seeked on input",
			params: SubtitleStreamParams{InputPath: "/cache/5.mkv", FromCache: true, StartSec: 42},
			want: []string{
				"-v", "error", "-nostdin",
				"-i", "/cache/5.mkv", "-copyts", "-map", "0:0", "-c:s", "copy",
				"-map_chapters", "-1", "-map_metadata:g", "-1", "-ss", "42.000",
				"-flush_packets", "1", "-f", "matroska", "pipe:1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := subtitleStreamArgs(tt.params)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("args =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestStreamSubtitleMatroskaRelaysStdoutAndRunsTheBuiltArguments(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "arguments.log")
	body := writeArgumentLog(logPath) + "printf 'matroska bytes'\n"
	script := writeFakeFFmpeg(t, "fake ffmpeg", body)
	f := &ffmpeg{bin: script}
	params := SubtitleStreamParams{InputPath: "/media/movie with spaces.mkv", StreamIndex: 3, StartSec: 12.5}

	stream, err := f.StreamSubtitleMatroska(context.Background(), params)
	if err != nil {
		t.Fatalf("StreamSubtitleMatroska: %v", err)
	}
	output, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	err = stream.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if string(output) != "matroska bytes" {
		t.Fatalf("output = %q", output)
	}
	args := readArgumentLog(t, logPath)
	if !slices.Equal(args, subtitleStreamArgs(params)) {
		t.Fatalf("FFmpeg ran with %q, want %q", args, subtitleStreamArgs(params))
	}
}

func TestStreamSubtitleMatroskaWaitReportsABoundedStderrTail(t *testing.T) {
	diagnostic := "discard-this-prefix-" + strings.Repeat("é", 3000) + "-diagnostic-tail"
	body := "printf 'partial'\nprintf '%s' '" + diagnostic + "' >&2\nexit 3\n"
	script := writeFakeFFmpeg(t, "fake ffmpeg", body)
	f := &ffmpeg{bin: script}

	stream, err := f.StreamSubtitleMatroska(context.Background(), SubtitleStreamParams{InputPath: "/movie.mkv", StreamIndex: 1})
	if err != nil {
		t.Fatalf("StreamSubtitleMatroska: %v", err)
	}
	_, err = io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	err = stream.Wait()
	if err == nil {
		t.Fatal("expected a stream failure")
	}
	message := err.Error()
	if !strings.Contains(message, "ffmpeg subtitle stream failed") || !strings.Contains(message, "diagnostic-tail") {
		t.Fatalf("error = %q, want the stream context and the stderr tail", message)
	}
	if strings.Contains(message, "discard-this-prefix") || !utf8.ValidString(message) {
		t.Fatalf("stderr tail was not bounded on a UTF-8 boundary: %d bytes", len(message))
	}
	if len(message) > 4300 {
		t.Fatalf("bounded diagnostic error is too large: %d bytes", len(message))
	}
}

func TestStreamSubtitleMatroskaCancellationEndsTheProcess(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", "printf 'header'\nexec sleep 30\n")
	f := &ffmpeg{bin: script}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := f.StreamSubtitleMatroska(ctx, SubtitleStreamParams{InputPath: "/movie.mkv", StreamIndex: 1})
	if err != nil {
		t.Fatalf("StreamSubtitleMatroska: %v", err)
	}
	header := make([]byte, len("header"))
	_, err = io.ReadFull(stream, header)
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	cancel()

	waited := make(chan error, 1)
	go func() {
		_, _ = io.Copy(io.Discard, stream)
		waited <- stream.Wait()
	}()
	select {
	case err = <-waited:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait error = %v, want context cancellation", err)
		}
	case <-time.After(testProcessTimeout):
		t.Fatal("subtitle stream did not end after cancellation")
	}
}

func TestExtractSubtitleMatroskaWritesASingleTrackWithSourceTimestamps(t *testing.T) {
	tests := []struct {
		name     string
		convert  bool
		maxBytes int64
		codec    string
		limit    []string
	}{
		{name: "copied", codec: "copy"},
		{name: "converted to SubRip", convert: true, codec: "srt"},
		{name: "capped", maxBytes: 1024, codec: "copy", limit: []string{"-fs", "1024"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "arguments.log")
			script := writeFakeFFmpeg(t, "fake ffmpeg", writeArgumentLog(logPath))
			f := &ffmpeg{bin: script}
			outputPath := filepath.Join(t.TempDir(), "track.tmp")

			err := f.ExtractSubtitleMatroska(context.Background(), SubtitleExtractParams{
				SourcePath:      "/media/movie.mkv",
				StreamIndex:     7,
				ConvertToSubRip: tt.convert,
				OutputPath:      outputPath,
				MaxBytes:        tt.maxBytes,
			})
			if err != nil {
				t.Fatalf("ExtractSubtitleMatroska: %v", err)
			}

			want := []string{
				"-v", "error", "-nostdin", "-y",
				"-i", "/media/movie.mkv", "-copyts", "-map", "0:7", "-c:s", tt.codec,
				"-map_chapters", "-1", "-map_metadata:g", "-1",
			}
			want = append(want, tt.limit...)
			want = append(want, "-f", "matroska", outputPath)
			args := readArgumentLog(t, logPath)
			if !slices.Equal(args, want) {
				t.Fatalf("args = %q, want %q", args, want)
			}
		})
	}
}

func TestExtractSubtitleMatroskaReportsFailure(t *testing.T) {
	script := writeFakeFFmpeg(t, "fake ffmpeg", "printf '%s\\n' read-failed >&2\nexit 1\n")
	f := &ffmpeg{bin: script}

	err := f.ExtractSubtitleMatroska(context.Background(), SubtitleExtractParams{SourcePath: "/movie.mkv", OutputPath: "/tmp/out.tmp"})
	if err == nil || !strings.Contains(err.Error(), "read-failed") {
		t.Fatalf("error = %v, want the FFmpeg diagnostic", err)
	}
}

func TestExtractCachedSubtitleAsWebVTTUsesTheMediaFilesTimeline(t *testing.T) {
	tests := []struct {
		name           string
		sourceStartSec float64
		offset         []string
	}{
		{name: "source starting at zero"},
		{name: "source starting later", sourceStartSec: 1.4, offset: []string{"-output_ts_offset", "-1.400000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "arguments.log")
			body := writeArgumentLog(logPath) + `
printf '%s\n' 'WEBVTT' '' '00:01.000 --> 00:02.000' 'a\hb'
`
			script := writeFakeFFmpeg(t, "fake ffmpeg", body)
			f := &ffmpeg{bin: script}

			output, err := f.ExtractCachedSubtitleAsWebVTT(context.Background(), "/cache/4.mkv", tt.sourceStartSec)
			if err != nil {
				t.Fatalf("ExtractCachedSubtitleAsWebVTT: %v", err)
			}
			if !strings.Contains(string(output), "a b") {
				t.Fatalf("hard-space normalization failed: %q", output)
			}
			want := []string{"-v", "error", "-nostdin", "-i", "/cache/4.mkv", "-copyts", "-map", "0:0"}
			want = append(want, tt.offset...)
			want = append(want, "-c:s", "webvtt", "-f", "webvtt", "pipe:1")
			args := readArgumentLog(t, logPath)
			if !slices.Equal(args, want) {
				t.Fatalf("args = %q, want %q", args, want)
			}
		})
	}
}
