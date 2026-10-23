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
// Returns 0 if the string is empty or cannot be parsed, and for negative, NaN or
// infinite rates: strconv.ParseFloat accepts "Inf" and "NaN", and callers already
// read 0 as an unknown rate rather than carrying a value they cannot compute with.
func ParseFrameRate(s string) float64 {
	numerator, denominator, isFraction := strings.Cut(s, "/")
	if !isFraction {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return 0
		}
		return finiteFrameRate(parsed)
	}

	num, numErr := strconv.ParseFloat(strings.TrimSpace(numerator), 64)
	den, denErr := strconv.ParseFloat(strings.TrimSpace(denominator), 64)
	if numErr != nil || denErr != nil || den == 0 {
		return 0
	}

	return finiteFrameRate(num / den)
}

func finiteFrameRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}

	return rate
}

// ParseDurationSeconds parses ffprobe's seconds-with-decimals duration string
// (e.g. "245.123456"). It reports false for empty, unparsable, negative, NaN or
// infinite input: strconv.ParseFloat accepts "Inf" and "NaN", and a duration of
// either is not something any caller can store or compute with.
func ParseDurationSeconds(s string) (float64, bool) {
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

// ParseSlashNumbers parses a "1/12" format string into its number and total.
// A bare number, or a total that does not parse, has a total of 0.
func ParseSlashNumbers(s string) (number, total int64, err error) {
	numberText, totalText, _ := strings.Cut(s, "/")
	if numberText == "" {
		return 0, 0, fmt.Errorf("invalid format: %s", s)
	}

	number, err = strconv.ParseInt(strings.TrimSpace(numberText), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	total, totalErr := strconv.ParseInt(strings.TrimSpace(totalText), 10, 64)
	if totalErr != nil {
		total = 0
	}

	return number, total, nil
}

// ParseBitRate returns 0 if parsing fails or the input is empty.
func ParseBitRate(bitRateStr string) int64 {
	parsed, err := strconv.ParseInt(bitRateStr, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// ParseDate attempts common audio metadata date formats.
func ParseDate(s string) (time.Time, error) {
	// Common date formats in audio metadata. A timestamp keeps the calendar
	// date it was written with: iTunes stores release dates as UTC instants
	// such as 2019-01-04T08:00:00Z, and callers format in the parsed zone.
	formats := []string{
		"2006-01-02",          // ISO 8601
		"2006-1-2",            // ISO 8601 without leading zeros (e.g. TMDB-style)
		"2006-01-02T15:04:05", // ISO 8601 with time
		time.RFC3339,          // ISO 8601 with time and zone (iTunes)
		"2006-01",             // Year and month
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
