package ffprobe

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// keyframeLookbackSec bounds how far before the requested offset ffprobe reads
// looking for a keyframe. Source GOPs on real library files run to roughly
// twelve seconds, so this leaves generous headroom without turning a bounded
// probe into a long read over a slow network mount.
const keyframeLookbackSec = 30.0

// KeyframeAtOrBefore returns the presentation time of the last keyframe at or
// before targetSec on the given absolute stream index.
//
// FFmpeg's input seek (-ss before -i) lands on the keyframe at or before the
// requested offset. When re-encoding it then discards frames up to the offset,
// so output starts exactly where asked; when copying video it cannot, so the
// session really begins at that earlier keyframe. This reports where.
func (f *ffprobe) KeyframeAtOrBefore(
	ctx context.Context,
	filePath string,
	streamIndex int64,
	targetSec float64,
) (float64, error) {
	if strings.TrimSpace(filePath) == "" {
		return 0, fmt.Errorf("source path is required")
	}

	if targetSec <= 0 {
		return 0, fmt.Errorf("target must be greater than zero")
	}

	from := targetSec - keyframeLookbackSec
	if from < 0 {
		from = 0
	}

	// Read from the lookback point up to one second past the target: the
	// keyframe we want is the last one at or before it.
	to := targetSec + 1

	args := []string{
		"-v", "error",
		"-select_streams", strconv.FormatInt(streamIndex, 10),
		"-read_intervals", fmt.Sprintf("%.3f%%%.3f", from, to),
		"-show_entries", "packet=pts_time,flags",
		"-of", "csv=p=0",
		filePath,
	}

	cmd := exec.CommandContext(ctx, f.bin, args...)

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("ffprobe keyframe lookup failed: %w", err)
	}

	best := -1.0
	for _, line := range strings.Split(string(out), "\n") {
		ptsTime, isKeyframe, ok := parseKeyframePacket(line)
		if !ok || !isKeyframe {
			continue
		}
		if ptsTime > targetSec {
			continue
		}
		if ptsTime > best {
			best = ptsTime
		}
	}

	if best < 0 {
		return 0, fmt.Errorf("no keyframe found at or before %.3f", targetSec)
	}

	return best, nil
}

// parseKeyframePacket reads one `pts_time,flags` CSV row. Rows with an unset
// pts (ffprobe writes "N/A") are skipped rather than treated as zero.
func parseKeyframePacket(line string) (float64, bool, bool) {
	fields := strings.Split(strings.TrimSpace(line), ",")
	if len(fields) < 2 {
		return 0, false, false
	}

	ptsTime, err := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
	if err != nil {
		return 0, false, false
	}

	isKeyframe := strings.Contains(fields[1], "K")

	return ptsTime, isKeyframe, true
}
