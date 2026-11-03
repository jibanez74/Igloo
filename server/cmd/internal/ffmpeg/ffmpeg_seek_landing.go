package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// SeekLandingParams names the source, video stream and start of a copy-video
// HLS session.
type SeekLandingParams struct {
	SourcePath       string
	VideoStreamIndex int
	StartSec         float64
}

// SeekLanding returns the time, in seconds relative to the container start, of
// the first video packet a copy-video HLS session starting at StartSec copies.
//
// FFmpeg's input seek does not simply land on the last keyframe at or before
// the start. When the video has B-frames, Matroska aims 3/23 s early, so a
// start just past a keyframe lands on the one before it, a whole GOP back. MP4
// picks keyframes by decode time, which can land a few frames after the start.
// Rather than model those rules, the same binary runs the same seek and copies
// a single packet with its timestamp normalized to the container start.
func (f *ffmpeg) SeekLanding(ctx context.Context, params SeekLandingParams) (float64, error) {
	if strings.TrimSpace(params.SourcePath) == "" {
		return 0, fmt.Errorf("source path is required")
	}
	if params.VideoStreamIndex < 0 {
		return 0, fmt.Errorf("video stream index must be non-negative")
	}
	if params.StartSec <= 0 {
		return 0, fmt.Errorf("start must be greater than zero")
	}

	cmd := exec.CommandContext(ctx, f.bin, seekLandingArgs(params)...)
	cmd.SysProcAttr = childProcessAttributes()
	var stdout bytes.Buffer
	// Only the tail of stderr reaches the error; keep no more than that.
	var stderr subtitleStderr
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return 0, commandError("ffmpeg seek measurement failed", ctx, err, stderr.String())
	}

	firstPTS := framecrcFirstPTS(stdout.Bytes())
	landing, found := firstPTS[0]
	if !found {
		return 0, errors.New("ffmpeg seek measurement copied no video packet")
	}
	return landing, nil
}

// seekLandingArgs repeat a copy-video session's probe and seek options, then
// copy one packet of its video stream with -copyts -start_at_zero, preserving
// its offset from the container start rather than its raw source timestamp.
func seekLandingArgs(params SeekLandingParams) []string {
	args := []string{"-v", "error", "-nostdin"}
	args = append(args, hlsProbeArgs()...)
	args = append(args, hlsSeekArgs(params.StartSec, true)...)
	return append(args,
		"-i", params.SourcePath,
		"-map", fmt.Sprintf("0:%d", params.VideoStreamIndex),
		"-c", "copy",
		"-copyts",
		"-start_at_zero",
		"-frames:v", "1",
		"-f", "framecrc",
		"pipe:1",
	)
}

// framecrcFirstPTS reads FFmpeg's framecrc listing and returns each output
// stream's first packet time in seconds: its presentation time, or its decode
// time when the packet has none.
func framecrcFirstPTS(listing []byte) map[int]float64 {
	timeBases := map[int]float64{}
	first := map[int]float64{}
	for line := range strings.Lines(string(listing)) {
		line = strings.TrimSpace(line)

		// "#tb 0: 1/1000"
		timeBaseLine, isTimeBase := strings.CutPrefix(line, "#tb ")
		if isTimeBase {
			index, fraction, _ := strings.Cut(timeBaseLine, ": ")
			num, den, _ := strings.Cut(fraction, "/")
			stream, indexErr := strconv.Atoi(index)
			numerator, numErr := strconv.ParseFloat(num, 64)
			denominator, denErr := strconv.ParseFloat(den, 64)
			valid := indexErr == nil && numErr == nil && denErr == nil && denominator > 0
			if valid {
				timeBases[stream] = numerator / denominator
			}
			continue
		}

		isPacket := line != "" && !strings.HasPrefix(line, "#")
		if !isPacket {
			continue
		}

		// "stream, dts, pts, duration, size, crc[, flags...]"
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		stream, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		_, seen := first[stream]
		timeBase, known := timeBases[stream]
		if seen || !known {
			continue
		}

		ts, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		noPTS := err != nil || ts == math.MinInt64
		if noPTS {
			ts, err = strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
			if err != nil || ts == math.MinInt64 {
				continue
			}
		}
		first[stream] = float64(ts) * timeBase
	}
	return first
}
