package movie

import (
	"testing"

	"igloo/cmd/internal/ffprobe"

	_ "github.com/mattn/go-sqlite3"
)

func TestChapterStartTimeSeconds(t *testing.T) {
	tests := []struct {
		name    string
		chapter ffprobe.Chapter
		want    int64
	}{
		{"prefers start_time seconds over raw ffprobe ticks", ffprobe.Chapter{StartTime: "573.114208", Start: 573114208}, 573},
		{"falls back to raw start when start_time missing", ffprobe.Chapter{Start: 12000}, 12},
		{"returns zero when chapter starts at zero", ffprobe.Chapter{StartTime: "0.000000"}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := chapterStartTimeSeconds(tc.chapter)
			if got != tc.want {
				t.Fatalf("chapterStartTimeSeconds() = %d, want %d", got, tc.want)
			}
		})
	}
}
