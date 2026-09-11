package scanner

import (
	"testing"

	"igloo/cmd/internal/ffprobe"
)

func TestChapterStartTimeSeconds(t *testing.T) {
	tests := []struct {
		name    string
		chapter ffprobe.Chapter
		want    int64
	}{
		{"prefers start_time seconds over raw ffprobe ticks", ffprobe.Chapter{StartTime: "573.114208"}, 573},
		{"returns zero when chapter starts at zero", ffprobe.Chapter{StartTime: "0.000000"}, 0},
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
