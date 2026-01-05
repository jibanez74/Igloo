package helpers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NullString returns a sql.NullString from a string.
// Returns an invalid NullString if the input is empty.
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}

	return sql.NullString{String: s, Valid: true}
}

// NullInt64 returns a sql.NullInt64 from an int64.
// Returns an invalid NullInt64 if the input is 0.
func NullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}

	return sql.NullInt64{Int64: i, Valid: true}
}

// NullFloat64 returns a sql.NullFloat64 from a float64.
// Returns an invalid NullFloat64 if the input is 0.
func NullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{Valid: false}
	}

	return sql.NullFloat64{Float64: f, Valid: true}
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

// ParseDate attempts to parse a date string in various common formats.
// Returns the parsed time or an error if none of the formats match.
func ParseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty string")
	}

	// Common date formats in audio metadata
	formats := []string{
		"2006-01-02",          // ISO 8601
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
