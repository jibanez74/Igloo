package helpers

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// NullString returns an invalid sql.NullString for empty input.
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}

	return sql.NullString{String: s, Valid: true}
}

// NullInt64 returns an invalid sql.NullInt64 for 0.
func NullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}

	return sql.NullInt64{Int64: i, Valid: true}
}

// NullFloat64 returns an invalid sql.NullFloat64 for 0.
func NullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{Valid: false}
	}

	return sql.NullFloat64{Float64: f, Valid: true}
}

// NullFloat64FromPtr returns an invalid sql.NullFloat64 for nil or 0.
func NullFloat64FromPtr(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}

	return NullFloat64(*f)
}

// StringPtrFromNull returns nil when the sql.NullString is invalid.
func StringPtrFromNull(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}

	return &s.String
}

// Float64PtrFromNull returns nil when the sql.NullFloat64 is invalid.
func Float64PtrFromNull(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}

	return &f.Float64
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

// ParseDurationMs converts a duration string (seconds with decimals) to milliseconds.
// Example: "245.123456" -> 245123
func ParseDurationMs(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}

	return int64(f * 1000), nil
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
// require finite values should validate before calling.
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
		"2006-01-02T15:04:05", // ISO 8601 with time, no zone
		time.RFC3339,          // ISO 8601 with time and zone (iTunes m4a "date")
		"2006-01-02 15:04:05", // ISO 8601 with time, space separator
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
