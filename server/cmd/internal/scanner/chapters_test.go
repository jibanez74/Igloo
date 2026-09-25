package scanner

import (
	"testing"

	"igloo/cmd/internal/ffprobe"
)

// Parsed start times are asserted through the movie and show persistence
// tests; only the fallbacks for missing or unparsable values live here.
func TestChapterStartTimeSeconds(t *testing.T) {
	tests := []struct {
		name    string
		chapter ffprobe.Chapter
		want    int64
	}{
		{"returns zero when start_time is absent", ffprobe.Chapter{}, 0},
		{"returns zero when start_time is unparsable", ffprobe.Chapter{StartTime: "not-a-number"}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ChapterStartTimeSeconds(tc.chapter)
			if got != tc.want {
				t.Fatalf("ChapterStartTimeSeconds() = %d, want %d", got, tc.want)
			}
		})
	}
}
