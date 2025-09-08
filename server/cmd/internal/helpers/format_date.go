package helpers

import (
	"fmt"
	"time"
)

var DateFormats = [5]string{
	"2006-01-02T15:04:05Z",             // ISO 8601 with Z timezone
	"2006-01-02T15:04:05.000000Z",      // ISO 8601 with microseconds and Z
	"2006-01-02T15:04:05-07:00",        // ISO 8601 with timezone offset
	"2006-01-02T15:04:05.000000-07:00", // ISO 8601 with microseconds and timezone offset
	"2006-01-02",                       // Date only
}

func FormatDate(date string) (time.Time, error) {
	for _, format := range DateFormats {
		if parsed, err := time.Parse(format, date); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", date)
}
