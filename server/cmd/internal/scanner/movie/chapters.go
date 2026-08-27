package movie

import (
	"context"
	"database/sql"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

func processChapters(ctx context.Context, qtx *database.Queries, movieID int64, chapters []ffprobe.Chapter) error {
	err := qtx.DeleteMovieChapters(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie chapters failed: %w", err)
	}

	for _, chapter := range chapters {
		_, err := qtx.InsertChapter(ctx, database.InsertChapterParams{
			MovieID:   movieID,
			Title:     chapter.Tags.Title,
			StartTime: chapterStartTimeSeconds(chapter),
			Thumb:     sql.NullString{},
		})
		if err != nil {
			return fmt.Errorf("insert chapter failed: %w", err)
		}
	}

	return nil
}

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
