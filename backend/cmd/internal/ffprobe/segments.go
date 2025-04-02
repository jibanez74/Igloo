package ffprobe

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// KeyframeData represents the keyframe information extracted from a video
type KeyframeData struct {
	TotalDuration time.Duration
	Keyframes     []time.Duration
}

// ExtractKeyframes extracts keyframe information from a video file
func (f *ffprobe) ExtractKeyframes(filePath string) (*KeyframeData, error) {
	durationCmd := exec.Command(f.bin,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "json",
		filePath)

	durationOutput, err := durationCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get duration: %w, output: %s", err, durationOutput)
	}

	var durationInfo struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	err = json.Unmarshal(durationOutput, &durationInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}

	var totalDuration time.Duration

	if durationInfo.Format.Duration != "" {
		duration, err := strconv.ParseFloat(durationInfo.Format.Duration, 64)
		if err == nil {
			totalDuration = time.Duration(duration * float64(time.Second))
		}
	}

	if totalDuration == 0 {
		return nil, fmt.Errorf("failed to get valid duration from video")
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-skip_frame", "nokey",
		"-select_streams", "v:0",
		"-show_entries", "frame=pts_time",
		"-of", "json",
		filePath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run ffprobe: %w, output: %s", err, output)
	}

	var frameData struct {
		Frames []struct {
			PtsTime string `json:"pts_time"`
		} `json:"frames"`
	}

	err = json.Unmarshal(output, &frameData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var keyframes []time.Duration

	for _, frame := range frameData.Frames {
		ptsTime, err := strconv.ParseFloat(frame.PtsTime, 64)
		if err != nil {
			continue
		}

		duration := time.Duration(ptsTime * float64(time.Second))
		keyframes = append(keyframes, duration)
	}

	if len(keyframes) == 0 {
		return nil, fmt.Errorf("no keyframes found in video")
	}

	return &KeyframeData{
		TotalDuration: totalDuration,
		Keyframes:     keyframes,
	}, nil
}

// ComputeSegments computes segment boundaries based on keyframe information
func ComputeSegments(keyframeData *KeyframeData, desiredSegmentLength time.Duration) []time.Duration {
	if keyframeData == nil {
		return nil
	}

	if len(keyframeData.Keyframes) == 0 {
		// If no keyframes, return equal length segments
		return computeEqualLengthSegments(keyframeData.TotalDuration, desiredSegmentLength)
	}

	var segments []time.Duration
	lastKeyframe := time.Duration(0)
	desiredCutTime := desiredSegmentLength

	for _, keyframe := range keyframeData.Keyframes {
		if keyframe >= desiredCutTime {
			// Add segment from last keyframe to current keyframe
			segments = append(segments, keyframe-lastKeyframe)
			lastKeyframe = keyframe
			desiredCutTime += desiredSegmentLength
		}
	}

	// Add final segment with remaining duration
	if lastKeyframe < keyframeData.TotalDuration {
		segments = append(segments, keyframeData.TotalDuration-lastKeyframe)
	}

	return segments
}

// computeEqualLengthSegments creates equal length segments when keyframe information is not available
func computeEqualLengthSegments(totalDuration, segmentLength time.Duration) []time.Duration {
	if totalDuration == 0 || segmentLength == 0 {
		return nil
	}

	wholeSegments := int(totalDuration / segmentLength)
	remainingDuration := totalDuration % segmentLength

	segments := make([]time.Duration, 0, wholeSegments+1)

	// Add whole segments
	for i := 0; i < wholeSegments; i++ {
		segments = append(segments, segmentLength)
	}

	// Add remaining duration if any
	if remainingDuration > 0 {
		segments = append(segments, remainingDuration)
	}

	return segments
}
