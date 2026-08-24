package main

import (
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
)

type capturedLogEntry struct {
	msg  string
	args []any
}

func newCapturedLogEntry(msg string, args []any) capturedLogEntry {
	entry := capturedLogEntry{
		msg:  msg,
		args: make([]any, len(args)),
	}
	copy(entry.args, args)

	return entry
}

type capturedLogger struct {
	debugEntries []capturedLogEntry
	infoEntries  []capturedLogEntry
}

func (l *capturedLogger) Debug(msg string, args ...any) {
	l.debugEntries = append(l.debugEntries, newCapturedLogEntry(msg, args))
}

func (l *capturedLogger) Info(msg string, args ...any) {
	l.infoEntries = append(l.infoEntries, newCapturedLogEntry(msg, args))
}

func (l *capturedLogger) Warn(_ string, _ ...any) {}

func (l *capturedLogger) Error(_ string, _ ...any) {}

func TestLogNormalizedChapterStartTimes(t *testing.T) {
	logger := &capturedLogger{}
	app := &Application{Logger: logger}
	movie := database.Movie{
		ID:    42,
		Title: "Short Film",
	}
	original := []database.Chapter{
		{ID: 7, StartTime: 3500},
		{ID: 8, StartTime: 12},
	}
	normalized := []database.Chapter{
		{ID: 7, StartTime: 300},
		{ID: 8, StartTime: 12},
	}

	app.logNormalizedChapterStartTimes(movie, original, normalized)

	if len(logger.debugEntries) != 1 {
		t.Fatalf("debug log count = %d, want 1", len(logger.debugEntries))
	}

	entry := logger.debugEntries[0]
	if entry.msg != "normalized chapter start time" {
		t.Fatalf("debug message = %q, want %q", entry.msg, "normalized chapter start time")
	}

	wantArgs := []any{
		"movie_id", int64(42),
		"movie_title", "Short Film",
		"chapter_id", int64(7),
		"chapter_index", 0,
		"original_start_time", int64(3500),
		"normalized_start_time", int64(300),
	}

	if len(entry.args) != len(wantArgs) {
		t.Fatalf("debug arg count = %d, want %d", len(entry.args), len(wantArgs))
	}

	for i := range wantArgs {
		if entry.args[i] != wantArgs[i] {
			t.Fatalf("debug arg %d = %#v, want %#v", i, entry.args[i], wantArgs[i])
		}
	}
}

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
			name:        "returns zero for non-positive start times",
			startTime:   0,
			durationSec: 8652.645,
			want:        0,
		},
		{
			name:        "clamps slightly overshot values to the movie duration",
			startTime:   9000,
			durationSec: 8652.645,
			want:        8653,
		},
		{
			name:        "clamps large values for short movies instead of shrinking them",
			startTime:   3500,
			durationSec: 300,
			want:        300,
		},
		{
			name:        "clamps very large values for very short movies",
			startTime:   1200,
			durationSec: 60,
			want:        60,
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
		{ID: 1, StartTime: 3500},
	}

	normalized := normalizeChaptersStartTimes(chapters, 300)

	if normalized[0].StartTime != 300 {
		t.Fatalf("normalized start_time = %d, want 300", normalized[0].StartTime)
	}

	if chapters[0].StartTime != 3500 {
		t.Fatalf("input slice mutated to %d", chapters[0].StartTime)
	}
}
