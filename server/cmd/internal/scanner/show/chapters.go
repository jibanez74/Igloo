package show

import (
	"context"
	"database/sql"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
)

func processChapters(ctx context.Context, qtx *database.Queries, fileID int64, chapters []ffprobe.Chapter) error {
	err := qtx.DeleteShowFileChapters(ctx, fileID)
	if err != nil {
		return fmt.Errorf("delete show chapters failed: %w", err)
	}

	for _, chapter := range chapters {
		err := qtx.InsertShowChapter(ctx, database.InsertShowChapterParams{
			FileID:    fileID,
			Title:     chapter.Tags.Title,
			StartTime: scanner.ChapterStartTimeSeconds(chapter),
			Thumb:     sql.NullString{},
		})
		if err != nil {
			return fmt.Errorf("insert chapter failed: %w", err)
		}
	}

	return nil
}
