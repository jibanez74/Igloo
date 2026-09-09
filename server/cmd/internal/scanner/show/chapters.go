package show

import (
	"context"
	"database/sql"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

func processChapters(ctx context.Context, qtx *database.Queries, fileID int64, chapters []ffprobe.Chapter) error {
	err := qtx.DeleteShowFileChapters(ctx, fileID)
	if err != nil {
		return fmt.Errorf("delete show chapters failed: %w", err)
	}

	for _, chapter := range chapters {
		_, err := qtx.InsertShowChapter(ctx, database.InsertShowChapterParams{
			FileID:    fileID,
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
