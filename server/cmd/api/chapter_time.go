package main

import (
	"math"

	"igloo/cmd/internal/database"
)

func normalizeChapterStartTimeSeconds(startTime int64, durationSec float64) int64 {
	if startTime <= 0 {
		return 0
	}

	if durationSec <= 0 {
		return startTime
	}

	limit := int64(math.Ceil(durationSec))
	if limit <= 0 {
		return startTime
	}

	if startTime > limit {
		return limit
	}

	return startTime
}

func normalizeChaptersStartTimes(
	chapters []database.Chapter,
	durationSec float64,
) []database.Chapter {
	if len(chapters) == 0 {
		return chapters
	}

	normalized := make([]database.Chapter, len(chapters))
	copy(normalized, chapters)

	for i := range normalized {
		normalized[i].StartTime = normalizeChapterStartTimeSeconds(
			normalized[i].StartTime,
			durationSec,
		)
	}

	return normalized
}
