package main

import (
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
)

func TestChapterStartTimeSeconds(t *testing.T) {
	tests := []struct {
		name    string
		chapter ffprobe.Chapter
		want    int64
	}{
		{
			name: "prefers start_time seconds over raw ffprobe ticks",
			chapter: ffprobe.Chapter{
				StartTime: "573.114208",
				Start:     573114208,
			},
			want: 573,
		},
		{
			name: "falls back to raw start when start_time missing",
			chapter: ffprobe.Chapter{
				Start: 12000,
			},
			want: 12,
		},
		{
			name: "returns zero when chapter starts at zero",
			chapter: ffprobe.Chapter{
				StartTime: "0.000000",
			},
			want: 0,
		},
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

func TestNormalizeChapterStartTimeSeconds(t *testing.T) {
	tests := []struct {
		name        string
		startTime   int64
		durationSec float64
		want        int64
	}{
		{
			name:        "leaves valid seconds unchanged",
			startTime:   573,
			durationSec: 8652.645,
			want:        573,
		},
		{
			name:        "normalizes legacy milliseconds",
			startTime:   573114,
			durationSec: 8652.645,
			want:        573,
		},
		{
			name:        "normalizes legacy microseconds",
			startTime:   573114208,
			durationSec: 8652.645,
			want:        573,
		},
		{
			name:        "clamps slightly overshot values to the movie duration",
			startTime:   9000,
			durationSec: 8652.645,
			want:        8653,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeChapterStartTimeSeconds(tc.startTime, tc.durationSec)
			if got != tc.want {
				t.Fatalf(
					"normalizeChapterStartTimeSeconds(%d, %.3f) = %d, want %d",
					tc.startTime,
					tc.durationSec,
					got,
					tc.want,
				)
			}
		})
	}
}

func TestNormalizeChaptersStartTimesReturnsCopy(t *testing.T) {
	chapters := []database.Chapter{
		{ID: 1, StartTime: 573114208},
	}

	normalized := normalizeChaptersStartTimes(chapters, 8652.645)

	if normalized[0].StartTime != 573 {
		t.Fatalf("normalized start_time = %d, want 573", normalized[0].StartTime)
	}

	if chapters[0].StartTime != 573114208 {
		t.Fatalf("input slice mutated to %d", chapters[0].StartTime)
	}
}
