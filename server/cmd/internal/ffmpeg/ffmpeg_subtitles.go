package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// subtitleStderrTailBytes caps the FFmpeg stderr tail carried in an extraction
// error, which is surfaced to the caller and logged. It is unrelated to the
// hlsStderrScanner* sizes in ffmpeg_hls.go — those bound one scanner token, not
// a tail kept after the fact.
const subtitleStderrTailBytes = 4096

// subtitleStreamWaitDelay bounds how long Wait holds on to the pipes of a
// subtitle stream FFmpeg has already left.
const subtitleStreamWaitDelay = 5 * time.Second

// SubtitleStreamParams selects one subtitle track to stream as Matroska.
type SubtitleStreamParams struct {
	// InputPath is the media file, or a cached single-track Matroska file.
	InputPath string
	// StreamIndex is the absolute stream index in InputPath; a cached track
	// carries its one stream at 0.
	StreamIndex int64
	// FromCache marks InputPath as a cached track. Its timestamps begin at its
	// first cue, which an input -ss would add to the seek, so it is read from
	// the start and the output -ss alone drops the earlier packets.
	FromCache bool
	// ConvertToSubRip re-encodes a text codec Matroska cannot carry.
	ConvertToSubRip bool
	// StartSec is the session start the packets are rebased onto; 0 streams
	// the whole track.
	StartSec float64
}

// SubtitleExtractParams copies one subtitle track of a media file into a
// single-track Matroska file.
type SubtitleExtractParams struct {
	SourcePath      string
	StreamIndex     int64
	ConvertToSubRip bool
	OutputPath      string
}

// SubtitleStream is a running FFmpeg writing one subtitle track as Matroska.
// Read returns its output; Wait, called once Read has reached EOF or the
// stream's context has ended, reports a failure with FFmpeg's stderr tail.
type SubtitleStream interface {
	io.Reader
	Wait() error
}

type subtitleStream struct {
	io.Reader

	ctx    context.Context
	cmd    *exec.Cmd
	stderr *subtitleStderr
}

func (s *subtitleStream) Wait() error {
	err := s.cmd.Wait()
	if err != nil {
		return subtitleCommandError("ffmpeg subtitle stream failed", s.ctx, err, s.stderr.String())
	}
	return nil
}

// subtitleStderr keeps the last subtitleStderrTailBytes of FFmpeg's stderr,
// so a process that runs for a whole film cannot grow it without bound. Only
// os/exec's copying goroutine writes; Wait returns after it is done.
type subtitleStderr struct {
	buf []byte
}

func (s *subtitleStderr) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	overflow := len(s.buf) - subtitleStderrTailBytes
	if overflow > 0 {
		s.buf = append(s.buf[:0:0], s.buf[overflow:]...)
	}
	return len(p), nil
}

func (s *subtitleStderr) String() string {
	return string(s.buf)
}

// ExtractSubtitleAsWebVTT converts one subtitle stream to WebVTT via ffmpeg.
// streamIndex is the absolute ffprobe index; the caller must reject bitmap codecs.
func (f *ffmpeg) ExtractSubtitleAsWebVTT(
	ctx context.Context,
	sourcePath string,
	streamIndex int64,
) ([]byte, error) {
	args := []string{
		"-v", "error",
		"-i", sourcePath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"pipe:1",
	}
	return f.runSubtitleWebVTT(ctx, args)
}

// ExtractCachedSubtitleAsWebVTT converts a cached text track to WebVTT. The
// cache keeps the source's timestamps, and -copyts keeps them in the cues.
func (f *ffmpeg) ExtractCachedSubtitleAsWebVTT(ctx context.Context, cachePath string) ([]byte, error) {
	args := []string{
		"-v", "error",
		"-nostdin",
		"-i", cachePath,
		"-copyts",
		"-map", "0:0",
		"-c:s", "webvtt",
		"-f", "webvtt",
		"pipe:1",
	}
	return f.runSubtitleWebVTT(ctx, args)
}

func (f *ffmpeg) runSubtitleWebVTT(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, f.bin, args...)
	cmd.SysProcAttr = childProcessAttributes()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, subtitleCommandError("ffmpeg subtitle extraction failed", ctx, err, stderr.String())
	}

	out := stdout.Bytes()
	out = bytes.ReplaceAll(out, []byte(`\h`), []byte(" "))

	return out, nil
}

// StreamSubtitleMatroska starts FFmpeg writing one subtitle track to its
// stdout as a non-seekable Matroska stream whose packet timestamps are the
// source's minus StartSec. Packets before StartSec are left out. The process
// ends with ctx.
func (f *ffmpeg) StreamSubtitleMatroska(ctx context.Context, params SubtitleStreamParams) (SubtitleStream, error) {
	cmd := exec.CommandContext(ctx, f.bin, subtitleStreamArgs(params)...)
	cmd.SysProcAttr = childProcessAttributes()
	cmd.WaitDelay = subtitleStreamWaitDelay
	stderr := &subtitleStderr{}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg subtitle stream pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("start ffmpeg subtitle stream: %w", err)
	}

	return &subtitleStream{Reader: stdout, ctx: ctx, cmd: cmd, stderr: stderr}, nil
}

// ExtractSubtitleMatroska copies one subtitle track of a media file, with the
// source's timestamps, into a single-track Matroska file at OutputPath,
// replacing any file there.
func (f *ffmpeg) ExtractSubtitleMatroska(ctx context.Context, params SubtitleExtractParams) error {
	args := []string{
		"-v", "error",
		"-nostdin",
		"-y",
		"-i", params.SourcePath,
		"-copyts",
		"-map", fmt.Sprintf("0:%d", params.StreamIndex),
		"-c:s", subtitleMatroskaCodec(params.ConvertToSubRip),
		"-map_chapters", "-1",
		"-map_metadata:g", "-1",
		"-f", "matroska",
		params.OutputPath,
	}

	cmd := exec.CommandContext(ctx, f.bin, args...)
	cmd.SysProcAttr = childProcessAttributes()
	stderr := &subtitleStderr{}
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return subtitleCommandError("ffmpeg subtitle track extraction failed", ctx, err, stderr.String())
	}
	return nil
}

// subtitleStreamArgs builds the stream's command line. Only global metadata is
// dropped: the track keeps its language and title, which a player reads to
// label and pick it. With -copyts, FFmpeg's
// output -ss drops packets before the start and subtracts it from the copied
// ones, which puts them on an HLS session's timeline exactly; the input -ss
// only skips ahead in a media file, compared against its raw timestamps
// (-seek_timestamp) as the output -ss is. An encoder ignores the output -ss
// shift, so converted text also takes -output_ts_offset.
func subtitleStreamArgs(params SubtitleStreamParams) []string {
	seeking := params.StartSec > 0
	start := strconv.FormatFloat(params.StartSec, 'f', 3, 64)

	args := []string{"-v", "error", "-nostdin"}
	seekInput := seeking && !params.FromCache
	if seekInput {
		args = append(args, "-seek_timestamp", "1", "-ss", start)
	}
	args = append(args,
		"-i", params.InputPath,
		"-copyts",
		"-map", fmt.Sprintf("0:%d", params.StreamIndex),
		"-c:s", subtitleMatroskaCodec(params.ConvertToSubRip),
		"-map_chapters", "-1",
		"-map_metadata:g", "-1",
	)
	if seeking {
		args = append(args, "-ss", start)
		if params.ConvertToSubRip {
			args = append(args, "-output_ts_offset", "-"+start)
		}
	}
	return append(args, "-flush_packets", "1", "-f", "matroska", "pipe:1")
}

func subtitleMatroskaCodec(convertToSubRip bool) string {
	if convertToSubRip {
		return "srt"
	}
	return "copy"
}

// subtitleCommandError wraps a failed subtitle FFmpeg run, preferring the
// context's error when the run was canceled, with a bounded stderr tail cut on
// a UTF-8 boundary.
func subtitleCommandError(action string, ctx context.Context, err error, stderr string) error {
	commandErr := err
	contextErr := ctx.Err()
	if contextErr != nil {
		commandErr = contextErr
	}
	tail := strings.TrimSpace(stderr)
	if len(tail) > subtitleStderrTailBytes {
		tail = tail[len(tail)-subtitleStderrTailBytes:]
	}
	for len(tail) > 0 && tail[0]&0xC0 == 0x80 {
		tail = tail[1:]
	}
	if tail != "" {
		return fmt.Errorf("%s: %w: %s", action, commandErr, tail)
	}
	return fmt.Errorf("%s: %w", action, commandErr)
}
