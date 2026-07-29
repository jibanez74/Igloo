package helpers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// webvttCueTiming matches the timing line of a WebVTT cue. WebVTT accepts both
// `MM:SS.mmm` and `HH:MM:SS.mmm`, and anything after the end timestamp is cue
// settings (`align:start`, `position:50%`, …) that must survive untouched.
var webvttCueTiming = regexp.MustCompile(
	`^((?:\d+:)?\d{2}:\d{2}\.\d{3})\s+-->\s+((?:\d+:)?\d{2}:\d{2}\.\d{3})(.*)$`,
)

// ShiftWebVTT rebases cue timings by offsetSec seconds.
//
// Subtitles are extracted from the source with absolute timestamps, but an HLS
// session started with `-ss` has a media timeline that begins at zero. Handing
// the browser absolute cues against a rebased timeline puts every subtitle out
// by the session start, so the cues are shifted to match the session they will
// be played against.
//
// Cues that end before the new zero are dropped; a cue straddling it is kept
// with its start clamped to zero. Headers, `NOTE`/`STYLE`/`REGION` blocks, cue
// identifiers and cue text all pass through unchanged.
func ShiftWebVTT(raw []byte, offsetSec float64) []byte {
	if len(raw) == 0 || offsetSec <= 0 {
		return raw
	}

	lines := strings.Split(string(raw), "\n")
	shifted := make([]string, 0, len(lines))

	// A dropped cue takes its identifier and text with it, so track whether the
	// block currently being copied belongs to a cue that fell before zero.
	droppingCue := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			droppingCue = false
			shifted = append(shifted, line)
			continue
		}

		match := webvttCueTiming.FindStringSubmatch(trimmed)
		if match == nil {
			if droppingCue {
				continue
			}
			shifted = append(shifted, line)
			continue
		}

		startSec, startErr := parseWebVTTTimestamp(match[1])
		endSec, endErr := parseWebVTTTimestamp(match[2])
		if startErr != nil || endErr != nil {
			shifted = append(shifted, line)
			continue
		}

		newEnd := endSec - offsetSec
		if newEnd < 0 {
			// The cue is entirely before the session start. Drop it, and drop
			// the identifier line already emitted for it.
			droppingCue = true
			shifted = dropTrailingCueIdentifier(shifted)
			continue
		}

		newStart := startSec - offsetSec
		if newStart < 0 {
			newStart = 0
		}

		droppingCue = false
		shifted = append(shifted, fmt.Sprintf(
			"%s --> %s%s",
			formatWebVTTTimestamp(newStart),
			formatWebVTTTimestamp(newEnd),
			match[3],
		))
	}

	return []byte(strings.Join(shifted, "\n"))
}

// dropTrailingCueIdentifier removes the optional identifier line that precedes
// a cue's timing line, so dropping a cue does not leave its label behind.
func dropTrailingCueIdentifier(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return lines
	}

	// Structural blocks are not cue identifiers and must be preserved.
	if strings.HasPrefix(last, "WEBVTT") ||
		strings.HasPrefix(last, "NOTE") ||
		strings.HasPrefix(last, "STYLE") ||
		strings.HasPrefix(last, "REGION") {
		return lines
	}

	return lines[:len(lines)-1]
}

func parseWebVTTTimestamp(value string) (float64, error) {
	parts := strings.Split(value, ":")

	var hours, minutes int
	var seconds float64
	var err error

	switch len(parts) {
	case 2:
		minutes, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		seconds, err = strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
	case 3:
		hours, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, err
		}
		seconds, err = strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("invalid WebVTT timestamp %q", value)
	}

	return float64(hours)*3600 + float64(minutes)*60 + seconds, nil
}

func formatWebVTTTimestamp(totalSec float64) string {
	if totalSec < 0 {
		totalSec = 0
	}

	// Round to milliseconds first so 59.9999 does not format as ":60.000".
	totalMillis := int64(totalSec*1000 + 0.5)

	hours := totalMillis / 3600000
	totalMillis -= hours * 3600000
	minutes := totalMillis / 60000
	totalMillis -= minutes * 60000
	seconds := totalMillis / 1000
	millis := totalMillis - seconds*1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}
