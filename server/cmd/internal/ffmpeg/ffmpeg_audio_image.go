package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func (f *ffmpeg) ExtractAudioImage(
	ctx context.Context,
	sourcePath string,
	streamIndex int64,
) ([]byte, error) {
	args := []string{
		"-v", "error",
		"-y",
		"-i", sourcePath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-frames:v", "1",
		"-c:v", "mjpeg",
		"-f", "image2",
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
			for len(tail) > 0 && tail[0]&0xC0 == 0x80 {
				tail = tail[1:]
			}
		}
		if tail != "" {
			return nil, fmt.Errorf("ffmpeg audio image extraction failed: %w: %s", err, tail)
		}
		return nil, fmt.Errorf("ffmpeg audio image extraction failed: %w", err)
	}

	out := stdout.Bytes()
	if len(out) == 0 {
		return nil, fmt.Errorf("ffmpeg audio image extraction produced no output")
	}

	return out, nil
}
