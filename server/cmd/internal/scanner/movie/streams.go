package movie

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

func processMovieStreams(
	ctx context.Context,
	qtx *database.Queries,
	movieID int64,
	streams []ffprobe.Stream,
) (videoStreamCount int, err error) {
	err = qtx.DeleteMovieVideoStreams(ctx, movieID)
	if err != nil {
		return 0, fmt.Errorf("delete movie video streams failed: %w", err)
	}
	err = qtx.DeleteMovieAudioStreams(ctx, movieID)
	if err != nil {
		return 0, fmt.Errorf("delete movie audio streams failed: %w", err)
	}
	err = qtx.DeleteMovieSubtitles(ctx, movieID)
	if err != nil {
		return 0, fmt.Errorf("delete movie subtitles failed: %w", err)
	}

	for _, stream := range streams {
		switch stream.CodecType {
		case "video":
			if stream.Disposition.AttachedPic == 1 {
				continue
			}
			if helpers.IsCoverArtVideoCodec(stream.CodecName) {
				continue
			}
			err = insertVideoStream(ctx, qtx, movieID, stream)
			if err != nil {
				return 0, err
			}
			videoStreamCount++
		case "audio":
			err = insertAudioStream(ctx, qtx, movieID, stream)
			if err != nil {
				return 0, err
			}
		case "subtitle":
			err = insertSubtitleStream(ctx, qtx, movieID, stream)
			if err != nil {
				return 0, err
			}
		}
	}

	return videoStreamCount, nil
}

func insertVideoStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	// An explicit 0-degree display matrix persists as 0 while absence persists
	// as NULL, so helpers.NullInt64 (which maps 0 to NULL) does not fit here.
	var rotation sql.NullInt64
	rotationDeg, hasRotation := stream.Rotation()
	if hasRotation {
		rotation = sql.NullInt64{Int64: rotationDeg, Valid: true}
	}

	err := qtx.InsertVideoStream(ctx, database.InsertVideoStreamParams{
		MovieID:        movieID,
		StreamIndex:    int64(stream.Index),
		Codec:          stream.CodecName,
		CodecProfile:   helpers.NullString(stream.Profile),
		CodecLevel:     helpers.NullInt64(int64(stream.Level)),
		BitRate:        helpers.ParseBitRate(stream.BitRate),
		Width:          int64(stream.Width),
		Height:         int64(stream.Height),
		CodedWidth:     helpers.NullInt64(int64(stream.CodedWidth)),
		CodedHeight:    helpers.NullInt64(int64(stream.CodedHeight)),
		AspectRatio:    helpers.NullString(stream.AspectRatio),
		FrameRate:      helpers.ParseFrameRate(stream.FrameRate),
		AvgFrameRate:   helpers.NullString(stream.AvgFrameRate),
		BitDepth:       parseNullInt64(stream.BitDepth),
		PixelFormat:    helpers.NullString(stream.PixelFormat),
		ColorRange:     helpers.NullString(stream.ColorRange),
		ColorSpace:     helpers.NullString(stream.ColorSpace),
		ColorPrimaries: helpers.NullString(stream.ColorPrimaries),
		ColorTransfer:  helpers.NullString(stream.ColorTransfer),
		FieldOrder:     helpers.NullString(stream.FieldOrder),
		Rotation:       rotation,
		Language:       helpers.NullString(stream.Tags.Language),
		Title:          helpers.NullString(stream.Tags.Title),
	})
	if err != nil {
		return fmt.Errorf("insert video stream failed: %w", err)
	}
	return nil
}

func insertAudioStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	err := qtx.InsertAudioStream(ctx, database.InsertAudioStreamParams{
		MovieID:       movieID,
		StreamIndex:   int64(stream.Index),
		Codec:         stream.CodecName,
		CodecProfile:  helpers.NullString(stream.Profile),
		BitRate:       helpers.ParseBitRate(stream.BitRate),
		SampleRate:    parseNullInt64(stream.SampleRate),
		Channels:      int64(stream.Channels),
		ChannelLayout: helpers.NullString(stream.ChannelLayout),
		Language:      helpers.NullString(stream.Tags.Language),
		Title:         helpers.NullString(stream.Tags.Title),
		IsDefault:     stream.Disposition.Default == 1,
	})
	if err != nil {
		return fmt.Errorf("insert audio stream failed: %w", err)
	}
	return nil
}

func insertSubtitleStream(ctx context.Context, qtx *database.Queries, movieID int64, stream ffprobe.Stream) error {
	err := qtx.InsertSubtitle(ctx, database.InsertSubtitleParams{
		MovieID:     movieID,
		StreamIndex: int64(stream.Index),
		Codec:       stream.CodecName,
		Language:    helpers.NullString(stream.Tags.Language),
		Title:       helpers.NullString(stream.Tags.Title),
		IsForced:    stream.Disposition.Forced == 1,
		IsDefault:   stream.Disposition.Default == 1,
	})
	if err != nil {
		return fmt.Errorf("insert subtitle failed: %w", err)
	}
	return nil
}

// parseNullInt64 maps ffprobe's optional numeric strings to NULL when absent
// or unparsable.
func parseNullInt64(value string) sql.NullInt64 {
	if value == "" {
		return sql.NullInt64{}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: parsed, Valid: true}
}
