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

func (s *Scanner) processMovieStreams(
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
	var codecLevel sql.NullInt64
	if stream.Level > 0 {
		codecLevel = sql.NullInt64{Int64: int64(stream.Level), Valid: true}
	}
	var bitDepth sql.NullInt64
	if stream.BitDepth != "" {
		parsed, err := strconv.ParseInt(stream.BitDepth, 10, 64)
		if err == nil {
			bitDepth = sql.NullInt64{Int64: parsed, Valid: true}
		}
	}
	var codedWidth, codedHeight sql.NullInt64
	if stream.CodedWidth > 0 {
		codedWidth = sql.NullInt64{Int64: int64(stream.CodedWidth), Valid: true}
	}
	if stream.CodedHeight > 0 {
		codedHeight = sql.NullInt64{Int64: int64(stream.CodedHeight), Valid: true}
	}
	// An explicit 0-degree display matrix persists as 0 while absence persists
	// as NULL, so helpers.NullInt64 (which maps 0 to NULL) does not fit here.
	var rotation sql.NullInt64
	rotationDeg, hasRotation := stream.Rotation()
	if hasRotation {
		rotation = sql.NullInt64{Int64: rotationDeg, Valid: true}
	}

	_, err := qtx.InsertVideoStream(ctx, database.InsertVideoStreamParams{
		MovieID:        movieID,
		StreamIndex:    int64(stream.Index),
		Codec:          stream.CodecName,
		CodecProfile:   helpers.NullString(stream.Profile),
		CodecLevel:     codecLevel,
		BitRate:        helpers.ParseBitRate(stream.BitRate),
		Width:          int64(stream.Width),
		Height:         int64(stream.Height),
		CodedWidth:     codedWidth,
		CodedHeight:    codedHeight,
		AspectRatio:    helpers.NullString(stream.AspectRatio),
		FrameRate:      helpers.ParseFrameRate(stream.FrameRate),
		AvgFrameRate:   helpers.NullString(stream.AvgFrameRate),
		BitDepth:       bitDepth,
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
	var sampleRate sql.NullInt64
	if stream.SampleRate != "" {
		parsed, err := strconv.ParseInt(stream.SampleRate, 10, 64)
		if err == nil {
			sampleRate = sql.NullInt64{Int64: parsed, Valid: true}
		}
	}

	_, err := qtx.InsertAudioStream(ctx, database.InsertAudioStreamParams{
		MovieID:       movieID,
		StreamIndex:   int64(stream.Index),
		Codec:         stream.CodecName,
		CodecProfile:  helpers.NullString(stream.Profile),
		BitRate:       helpers.ParseBitRate(stream.BitRate),
		SampleRate:    sampleRate,
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
	_, err := qtx.InsertSubtitle(ctx, database.InsertSubtitleParams{
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
