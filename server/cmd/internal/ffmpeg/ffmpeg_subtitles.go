package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// subtitleStderrTailBytes caps the FFmpeg stderr tail carried in an extraction
// error, matching the hlsStderrScanner* caps in ffmpeg_hls.go.
const subtitleStderrTailBytes = 4096

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

	cmd := exec.CommandContext(ctx, f.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		commandErr := err
		contextErr := ctx.Err()
		if contextErr != nil {
			commandErr = contextErr
		}
		tail := strings.TrimSpace(stderr.String())
		if len(tail) > subtitleStderrTailBytes {
			tail = tail[len(tail)-subtitleStderrTailBytes:]
			for len(tail) > 0 && tail[0]&0xC0 == 0x80 {
				tail = tail[1:]
			}
		}
		if tail != "" {
			return nil, fmt.Errorf("ffmpeg subtitle extraction failed: %w: %s", commandErr, tail)
		}
		return nil, fmt.Errorf("ffmpeg subtitle extraction failed: %w", commandErr)
	}

	out := stdout.Bytes()
	out = bytes.ReplaceAll(out, []byte(`\h`), []byte(" "))

	return out, nil
}
