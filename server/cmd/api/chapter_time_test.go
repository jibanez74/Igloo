package main

import (
	"testing"

	"igloo/cmd/internal/database"
)

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
