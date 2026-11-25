package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// pictureTimeout bounds one extraction: a stream copy of a single packet.
const pictureTimeout = 60 * time.Second

// ExtractAttachedPicture copies the picture an audio file embeds as stream
// streamIndex, an attached picture, to destPath as it is stored: a JPEG or
// PNG packet needs no encoder, and the image2 muxer writes its bytes
// unchanged. destPath is overwritten, so the caller writes to a temporary
// name of its own and checks the bytes before using them.
func (f *ffmpeg) ExtractAttachedPicture(ctx context.Context, sourcePath string, streamIndex int, destPath string) error {
	if strings.TrimSpace(sourcePath) == "" || strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("source and destination paths are required")
	}
	if streamIndex < 0 {
		return fmt.Errorf("stream index must be non-negative")
	}

	extractCtx, cancel := context.WithTimeout(ctx, pictureTimeout)
	defer cancel()
	cmd := exec.CommandContext(extractCtx, f.bin, attachedPictureArgs(sourcePath, streamIndex, destPath)...)
	cmd.SysProcAttr = childProcessAttributes()
	var stderr subtitleStderr
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return commandError("ffmpeg picture extraction failed", extractCtx, err, stderr.String())
	}
	return nil
}

// attachedPictureArgs map the one picture stream and copy its single packet
// into one image file; -update 1 names that file rather than a sequence.
func attachedPictureArgs(sourcePath string, streamIndex int, destPath string) []string {
	return []string{
		"-hide_banner", "-nostdin", "-v", "error", "-y",
		"-i", sourcePath,
		"-map", "0:" + strconv.Itoa(streamIndex),
		"-c", "copy", "-frames:v", "1",
		"-f", "image2", "-update", "1",
		destPath,
	}
}
