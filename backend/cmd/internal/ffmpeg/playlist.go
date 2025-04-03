package ffmpeg

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type playlistInfo struct {
	Version        int
	TargetDuration float64
	MediaSequence  int
	Segments       []segmentInfo
	InitSegment    string
	KeyframeBased  bool    // Indicates if segments are keyframe-based
	MaxDuration    float64 // Maximum allowed segment duration
	MinDuration    float64 // Minimum allowed segment duration
}

type segmentInfo struct {
	Duration   float64
	URI        string
	IsKeyframe bool    // Indicates if this segment starts with a keyframe
	StartTime  float64 // Start time of the segment in the video
	EndTime    float64 // End time of the segment in the video
}

func (f *ffmpeg) convertToVod(playlistPath string) error {
	content, err := os.ReadFile(playlistPath)
	if err != nil {
		return &ffmpegError{
			Field: "playlist",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to read playlist: %v", err),
		}
	}

	info, err := f.parsePlaylist(string(content))
	if err != nil {
		return &ffmpegError{
			Field: "playlist",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to parse playlist: %v", err),
		}
	}

	vodContent := f.generateVodPlaylist(info)

	err = os.WriteFile(playlistPath, []byte(vodContent), 0644)
	if err != nil {
		return &ffmpegError{
			Field: "playlist",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to write VOD playlist: %v", err),
		}
	}

	return nil
}

func (f *ffmpeg) parsePlaylist(content string) (*playlistInfo, error) {
	info := &playlistInfo{
		Version:       7,
		Segments:      make([]segmentInfo, 0),
		KeyframeBased: true, // Default to keyframe-based segmentation
		MaxDuration:   10.0, // Maximum segment duration in seconds
		MinDuration:   2.0,  // Minimum segment duration in seconds
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	currentTime := 0.0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "#EXT-X-VERSION:"):
			version, err := strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-VERSION:"))
			if err != nil {
				return nil, &ffmpegError{
					Field: "version",
					Value: strings.TrimPrefix(line, "#EXT-X-VERSION:"),
					Msg:   "invalid version format",
				}
			}
			info.Version = version

		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
			duration, err := strconv.ParseFloat(strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"), 64)
			if err != nil {
				return nil, &ffmpegError{
					Field: "target_duration",
					Value: strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"),
					Msg:   "invalid duration format",
				}
			}
			info.TargetDuration = duration

		case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
			sequence, err := strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"))
			if err != nil {
				return nil, &ffmpegError{
					Field: "media_sequence",
					Value: strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"),
					Msg:   "invalid sequence format",
				}
			}
			info.MediaSequence = sequence

		case strings.HasPrefix(line, "#EXT-X-MAP:"):
			uri := f.extractUriFromMap(line)
			info.InitSegment = uri

		case strings.HasPrefix(line, "#EXTINF:"):
			durationStr := strings.TrimPrefix(line, "#EXTINF:")
			durationStr = strings.TrimSuffix(durationStr, ",")
			duration, err := strconv.ParseFloat(durationStr, 64)
			if err != nil {
				return nil, &ffmpegError{
					Field: "segment_duration",
					Value: durationStr,
					Msg:   "invalid segment duration format",
				}
			}

			// Check if duration is within allowed range
			if duration > info.MaxDuration {
				return nil, &ffmpegError{
					Field: "segment_duration",
					Value: fmt.Sprintf("%.3f", duration),
					Msg:   fmt.Sprintf("segment duration exceeds maximum allowed duration of %.1f seconds", info.MaxDuration),
				}
			}

			if duration < info.MinDuration {
				return nil, &ffmpegError{
					Field: "segment_duration",
					Value: fmt.Sprintf("%.3f", duration),
					Msg:   fmt.Sprintf("segment duration is below minimum allowed duration of %.1f seconds", info.MinDuration),
				}
			}

			if !scanner.Scan() {
				return nil, &ffmpegError{
					Field: "segment_uri",
					Value: "missing",
					Msg:   "missing segment URI after duration",
				}
			}
			uri := strings.TrimSpace(scanner.Text())

			// Check for keyframe indicator in URI (if present)
			isKeyframe := strings.Contains(uri, "#keyframe")
			if isKeyframe {
				uri = strings.TrimSuffix(uri, "#keyframe")
			}

			segment := segmentInfo{
				Duration:   duration,
				URI:        uri,
				IsKeyframe: isKeyframe,
				StartTime:  currentTime,
				EndTime:    currentTime + duration,
			}

			info.Segments = append(info.Segments, segment)
			currentTime += duration
		}
	}

	return info, nil
}

func (f *ffmpeg) generateVodPlaylist(info *playlistInfo) string {
	var builder strings.Builder

	builder.WriteString("#EXTM3U\n")
	builder.WriteString(fmt.Sprintf("#EXT-X-VERSION:%d\n", info.Version))
	builder.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	builder.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%.0f\n", info.TargetDuration))
	builder.WriteString(fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", info.MediaSequence))

	if info.InitSegment != "" {
		builder.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s\"\n", info.InitSegment))
	}

	for _, segment := range info.Segments {
		// Add keyframe indicator if present
		uri := segment.URI
		if segment.IsKeyframe {
			uri += "#keyframe"
		}

		builder.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", segment.Duration))
		builder.WriteString(uri + "\n")
	}

	builder.WriteString("#EXT-X-ENDLIST\n")

	return builder.String()
}

func (f *ffmpeg) extractUriFromMap(line string) string {
	line = strings.TrimPrefix(line, "#EXT-X-MAP:")

	if strings.Contains(line, "URI=") {
		parts := strings.Split(line, "URI=")
		if len(parts) > 1 {
			// Find the first quote
			start := strings.Index(parts[1], "\"")
			if start == -1 {
				return ""
			}
			// Find the next quote after the first one
			end := strings.Index(parts[1][start+1:], "\"")
			if end == -1 {
				return ""
			}
			// Adjust end index to account for the substring
			end = start + 1 + end
			return parts[1][start+1 : end]
		}
	}

	return ""
}

func (f *ffmpeg) getSegmentCount(playlistPath string) (int, error) {
	content, err := os.ReadFile(playlistPath)
	if err != nil {
		return 0, &ffmpegError{
			Field: "playlist",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to read playlist: %v", err),
		}
	}

	info, err := f.parsePlaylist(string(content))
	if err != nil {
		return 0, &ffmpegError{
			Field: "playlist",
			Value: playlistPath,
			Msg:   fmt.Sprintf("failed to parse playlist: %v", err),
		}
	}

	return len(info.Segments), nil
}

func (f *ffmpeg) waitForSegments(playlistPath string, minSegments int, timeout time.Duration) error {
	startTime := time.Now()
	for {
		count, err := f.getSegmentCount(playlistPath)
		if err != nil {
			return err
		}

		if count >= minSegments {
			return nil
		}

		if time.Since(startTime) > timeout {
			return &ffmpegError{
				Field: "timeout",
				Value: fmt.Sprintf("%v", timeout),
				Msg:   fmt.Sprintf("timed out waiting for %d segments", minSegments),
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}
