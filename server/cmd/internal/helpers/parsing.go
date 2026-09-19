package helpers

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// normalizeCodec puts an ffprobe codec name into the form the lookup tables in
// this package are keyed by. Every codec predicate goes through it so that
// whitespace and casing cannot make two of them disagree about one stream.
func normalizeCodec(codec string) string {
	return strings.ToLower(strings.TrimSpace(codec))
}

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

// ParseDurationSeconds parses ffprobe's seconds-with-decimals duration string
// (e.g. "245.123456"). It reports false for empty, unparsable, negative, NaN or
// infinite input: strconv.ParseFloat accepts "Inf" and "NaN", and a duration of
// either is not something any caller can store or compute with.
func ParseDurationSeconds(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}

	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0, false
	}

	return seconds, true
}

// ParseDurationMs is ParseDurationSeconds expressed in milliseconds.
// Example: "245.123456" -> 245123
func ParseDurationMs(s string) (int64, error) {
	seconds, ok := ParseDurationSeconds(s)
	if !ok {
		return 0, fmt.Errorf("invalid duration: %q", s)
	}

	return int64(seconds * 1000), nil
}

// ParseSlashNumber parses a "1/12" format string and returns the first number.
// Used for parsing track numbers and disc numbers from metadata.
func ParseSlashNumber(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	parts := strings.Split(s, "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, fmt.Errorf("invalid format: %s", s)
	}

	return strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
}

// ParseBitRate returns 0 if parsing fails or the input is empty.
func ParseBitRate(bitRateStr string) int64 {
	if bitRateStr == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(bitRateStr, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// ClampFloat64 returns v limited to [min, max]. If min > max, the bounds are swapped.
// If v, min, or max is NaN, the result follows IEEE 754 (NaN propagates); callers that
// require finite values should validate before calling. It lives beside the parsers as
// the range guard applied to the values they produce.
func ClampFloat64(v, min, max float64) float64 {
	if min > max {
		min, max = max, min
	}
	return math.Min(math.Max(v, min), max)
}

// ParseDate attempts common audio metadata date formats.
func ParseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty string")
	}

	// Common date formats in audio metadata
	formats := []string{
		"2006-01-02",          // ISO 8601
		"2006-1-2",            // ISO 8601 without leading zeros (e.g. TMDB-style)
		"2006-01-02T15:04:05", // ISO 8601 with time
		"2006",                // Year only
		"01/02/2006",          // US format
		"02-01-2006",          // European format
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}
