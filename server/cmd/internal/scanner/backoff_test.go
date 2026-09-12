package scanner

import (
	"database/sql"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
)

func TestMissBackoffElapsed(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	missedAt := helpers.NullInt64(now.Add(-time.Hour).Unix())
	cases := []struct {
		name          string
		attempts      int64
		lastAttemptAt sql.NullInt64
		at            time.Time
		want          bool
	}{
		{"never attempted", 0, sql.NullInt64{}, now, true},
		{"first miss retries next scan", 1, missedAt, now, true},
		{"second miss waits a day", 2, missedAt, now, false},
		{"second miss eligible after a day", 2, missedAt, now.Add(24 * time.Hour), true},
		{"third miss waits two days", 3, missedAt, now.Add(24 * time.Hour), false},
		{"backoff caps at a week", 40, missedAt, now.Add(7 * 24 * time.Hour), true},
		{"missing timestamp retries", 5, sql.NullInt64{}, now, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MissBackoffElapsed(tc.attempts, tc.lastAttemptAt, tc.at)
			if got != tc.want {
				t.Fatalf("elapsed=%v want=%v", got, tc.want)
			}
		})
	}
}
