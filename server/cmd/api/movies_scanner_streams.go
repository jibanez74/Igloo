package main

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"strconv"
)

// processMovieStreams processes all video, audio, and subtitle streams from FFPROBE data
// in a single pass. Returns the number of video streams processed and an error.
func (app *Application) processMovieStreams(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	streams []ffprobe.Stream,
) (videoStreamCount int, err error) {
	if err := qtx.DeleteMovieVideoStreams(ctx, movieID); err != nil {
		return 0, fmt.Errorf("delete movie video streams failed: %w", err)
	}
	if err := qtx.DeleteMovieAudioStreams(ctx, movieID); err != nil {
		return 0, fmt.Errorf("delete movie audio streams failed: %w", err)
	}
	if err := qtx.DeleteMovieSubtitles(ctx, movieID); err != nil {
		return 0, fmt.Errorf("delete movie subtitles failed: %w", err)
	}

	for _, stream := range streams {
		switch stream.CodecType {
		case "video":
			if err := app.insertVideoStream(ctx, qtx, movieID, stream); err != nil {
				return 0, err
			}
			videoStreamCount++
		case "audio":
			if err := app.insertAudioStream(ctx, qtx, movieID, stream); err != nil {
				return 0, err
			}
		case "subtitle":
			if err := app.insertSubtitleStream(ctx, qtx, movieID, stream); err != nil {
				return 0, err
			}
		}
	}

	return videoStreamCount, nil
}

func (app *Application) insertVideoStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	bitRate := helpers.ParseBitRate(stream.BitRate)
	var codecLevel sql.NullInt64
	if stream.Level > 0 {
		codecLevel = sql.NullInt64{Int64: int64(stream.Level), Valid: true}
	}
	var bitDepth sql.NullInt64
	if stream.BitDepth != "" {
		if parsed, err := strconv.ParseInt(stream.BitDepth, 10, 64); err == nil {
			bitDepth = sql.NullInt64{Int64: parsed, Valid: true}
		}
	}
	frameRate := helpers.ParseFrameRate(stream.FrameRate)
	var codedWidth, codedHeight sql.NullInt64
	if stream.CodedWidth > 0 {
		codedWidth = sql.NullInt64{Int64: int64(stream.CodedWidth), Valid: true}
	}
	if stream.CodedHeight > 0 {
		codedHeight = sql.NullInt64{Int64: int64(stream.CodedHeight), Valid: true}
	}

	_, err := qtx.InsertVideoStream(ctx, database.InsertVideoStreamParams{
		MovieID:        movieID,
		StreamIndex:    int64(stream.Index),
		Codec:          stream.CodecName,
		CodecProfile:   helpers.NullString(stream.Profile),
		CodecLevel:     codecLevel,
		BitRate:        bitRate,
		Width:          int64(stream.Width),
		Height:         int64(stream.Height),
		CodedWidth:     codedWidth,
		CodedHeight:    codedHeight,
		AspectRatio:    helpers.NullString(stream.AspectRatio),
		FrameRate:      frameRate,
		AvgFrameRate:   helpers.NullString(stream.AvgFrameRate),
		BitDepth:       bitDepth,
		ColorRange:     helpers.NullString(stream.ColorRange),
		ColorSpace:     helpers.NullString(stream.ColorSpace),
		ColorPrimaries: helpers.NullString(stream.ColorPrimaries),
		ColorTransfer:  helpers.NullString(stream.ColorTransfer),
		Language:       helpers.NullString(stream.Tags.Language),
		Title:          helpers.NullString(stream.Tags.Title),
	})
	if err != nil {
		return fmt.Errorf("insert video stream failed: %w", err)
	}
	return nil
}

func (app *Application) insertAudioStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	bitRate := helpers.ParseBitRate(stream.BitRate)
	var sampleRate sql.NullInt64
	if stream.SampleRate != "" {
		if parsed, err := strconv.ParseInt(stream.SampleRate, 10, 64); err == nil {
			sampleRate = sql.NullInt64{Int64: parsed, Valid: true}
		}
	}
	_, err := qtx.InsertAudioStream(ctx, database.InsertAudioStreamParams{
		MovieID:       movieID,
		StreamIndex:   int64(stream.Index),
		Codec:         stream.CodecName,
		CodecProfile:  helpers.NullString(stream.Profile),
		BitRate:       bitRate,
		SampleRate:    sampleRate,
		Channels:      int64(stream.Channels),
		ChannelLayout: helpers.NullString(stream.ChannelLayout),
		Language:      helpers.NullString(stream.Tags.Language),
		Title:         helpers.NullString(stream.Tags.Title),
	})
	if err != nil {
		return fmt.Errorf("insert audio stream failed: %w", err)
	}
	return nil
}

func (app *Application) insertSubtitleStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	_, err := qtx.InsertSubtitle(ctx, database.InsertSubtitleParams{
		MovieID:     movieID,
		StreamIndex: int64(stream.Index),
		Codec:       stream.CodecName,
		Language:    helpers.NullString(stream.Tags.Language),
		Title:       helpers.NullString(stream.Tags.Title),
		IsForced:    false,
		IsDefault:   false,
	})
	if err != nil {
		return fmt.Errorf("insert subtitle failed: %w", err)
	}
	return nil
}

// processChapters processes chapters from FFPROBE data.
func (app *Application) processChapters(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	chapters []ffprobe.Chapter,
) error {
	// Delete all existing chapters
	if err := qtx.DeleteMovieChapters(ctx, helpers.NullInt64(movieID)); err != nil {
		return fmt.Errorf("delete movie chapters failed: %w", err)
	}

	for _, chapter := range chapters {
		// Parse start time (convert from seconds to milliseconds)
		startTime := int64(0)
		if chapter.Start > 0 {
			startTime = int64(chapter.Start)
		} else if chapter.StartTime != "" {
			// Parse duration string (e.g., "123.456")
			if duration, err := helpers.ParseDurationMs(chapter.StartTime); err == nil {
				startTime = duration
			}
		}

		// Leave thumb empty (chapter thumbnail generation will be implemented later)
		_, err := qtx.InsertChapter(ctx, database.InsertChapterParams{
			MovieID:   helpers.NullInt64(movieID),
			Title:     chapter.Tags.Title,
			StartTime: startTime,
			Thumb:     sql.NullString{}, // Empty for now
		})
		if err != nil {
			return fmt.Errorf("insert chapter failed: %w", err)
		}
	}

	return nil
}
