package helpers

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		value  string
		want   time.Duration
		wantOK bool
	}{
		{"missing", "", 0, false},
		{"seconds", "3", 3 * time.Second, true},
		{"seconds with spaces", " 7 ", 7 * time.Second, true},
		{"zero seconds", "0", 0, true},
		{"negative seconds", "-1", 0, false},
		{"future date", now.Add(4 * time.Second).Format(http.TimeFormat), 4 * time.Second, true},
		{"past date", now.Add(-time.Minute).Format(http.TimeFormat), 0, true},
		{"garbage", "soon", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.value != "" {
				headers.Set("Retry-After", tt.value)
			}
			got, ok := RetryAfter(headers, now)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("RetryAfter(%q) = %v, %v; want %v, %v", tt.value, got, ok, tt.want, tt.wantOK)
			}
		})
	}

	if _, ok := RetryAfter(nil, now); ok {
		t.Fatal("RetryAfter(nil) reported a value")
	}
}

func TestSleepContext(t *testing.T) {
	err := SleepContext(context.Background(), time.Millisecond)
	if err != nil {
		t.Fatalf("SleepContext returned %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = SleepContext(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SleepContext on a canceled context returned %v, want context.Canceled", err)
	}
	err = SleepContext(ctx, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SleepContext(0) on a canceled context returned %v, want context.Canceled", err)
	}
}
