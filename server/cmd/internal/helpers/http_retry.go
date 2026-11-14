package helpers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryAfter reads a Retry-After header, given either in seconds or as an HTTP
// date. ok is false when the header is missing or unreadable; a date already
// past reads as zero. headers may be nil.
func RetryAfter(headers http.Header, now time.Time) (time.Duration, bool) {
	value := strings.TrimSpace(headers.Get("Retry-After"))
	if value == "" {
		return 0, false
	}

	seconds, err := strconv.Atoi(value)
	if err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	retryTime, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	return max(retryTime.Sub(now), 0), true
}

// SleepContext waits for delay or until ctx is done, returning ctx's error in
// the second case. A delay of zero or less only reports ctx's state.
func SleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
