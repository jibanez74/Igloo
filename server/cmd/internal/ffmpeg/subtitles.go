package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ExtractSubtitleAsWebVTT converts one subtitle stream to WebVTT via ffmpeg.
// streamIndex is the absolute ffprobe index; the caller must reject bitmap codecs.
func (f *FFmpeg) ExtractSubtitleAsWebVTT(
	ctx context.Context,
	sourcePath string,
	streamIndex int64,
) ([]byte, error) {
	args := []string{
		"-y",
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
		tail := strings.TrimSpace(stderr.String())
		if len(tail) > 4096 {
			tail = tail[len(tail)-4096:]
		}
		if tail != "" {
			return nil, fmt.Errorf("ffmpeg subtitle extraction failed: %w: %s", err, tail)
		}
		return nil, fmt.Errorf("ffmpeg subtitle extraction failed: %w", err)
	}

	out := stdout.Bytes()
	out = bytes.ReplaceAll(out, []byte(`\h`), []byte(" "))

	return out, nil
}
