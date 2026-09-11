package movie

import (
	"context"
	"database/sql"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
)

func processChapters(ctx context.Context, qtx *database.Queries, movieID int64, chapters []ffprobe.Chapter) error {
	err := qtx.DeleteMovieChapters(ctx, movieID)
	if err != nil {
		return fmt.Errorf("delete movie chapters failed: %w", err)
	}

	for _, chapter := range chapters {
		err := qtx.InsertChapter(ctx, database.InsertChapterParams{
			MovieID:   movieID,
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
