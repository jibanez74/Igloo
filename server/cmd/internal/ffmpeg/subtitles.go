package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// ExtractSubtitleAsWebVTT runs ffmpeg to extract a single subtitle stream
// from sourcePath and convert it to WebVTT. streamIndex is the absolute
// ffprobe stream index (not a type-relative index). The full WebVTT output
// is returned as a byte slice.
//
// Supported input codecs: subrip, mov_text, webvtt, eia_608, ass, ssa.
// Bitmap codecs (PGS, DVD sub) must be rejected by the caller before
// invoking this function.
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
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg subtitle extraction failed: %w", err)
	}

	// ffmpeg leaves ASS-style \h (non-breaking space) escapes in WebVTT
	// output for some codecs (notably eia_608). Replace with regular spaces.
			out = bytes.ReplaceAll(out, []byte(`\h`), []byte(" "))

	return out, nil
}
