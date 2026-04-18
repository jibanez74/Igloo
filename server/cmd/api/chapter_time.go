package main

import (
	"math"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

func chapterStartTimeSeconds(chapter ffprobe.Chapter) int64 {
	if chapter.StartTime != "" {
		durationMs, err := helpers.ParseDurationMs(chapter.StartTime)
		if err == nil {
			return durationMs / 1000
		}
	}

	if chapter.Start > 0 {
		return int64(chapter.Start) / 1000
	}

	return 0
}

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
