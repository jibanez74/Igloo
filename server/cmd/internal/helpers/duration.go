package helpers

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseFrameRate parses an FFPROBE-style frame rate string into frames per second.
// Supports fraction form (e.g. "24000/1001") and decimal form (e.g. "23.976").
// Returns 0 if the string is empty or cannot be parsed.
func ParseFrameRate(s string) float64 {
	if s == "" {
		return 0
	}
	if strings.Contains(s, "/") {
		parts := strings.Split(s, "/")
		if len(parts) != 2 {
			return 0
		}
		num, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		den, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0
		}
		return num / den
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return parsed
}

// FormatDuration returns a human-readable duration string.
// Examples: "1.5s", "2m 30s", "1h 15m"
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
