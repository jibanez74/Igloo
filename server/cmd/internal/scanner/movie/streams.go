package movie

import (
	"context"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
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

	return scanner.ClassifyStreams(streams,
		func(v scanner.VideoStreamFields) error {
			err := qtx.InsertVideoStream(ctx, database.InsertVideoStreamParams{
				MovieID: movieID, StreamIndex: v.StreamIndex, Codec: v.Codec, CodecProfile: v.CodecProfile, CodecLevel: v.CodecLevel,
				BitRate: v.BitRate, Width: v.Width, Height: v.Height, CodedWidth: v.CodedWidth, CodedHeight: v.CodedHeight,
				AspectRatio: v.AspectRatio, FrameRate: v.FrameRate, AvgFrameRate: v.AvgFrameRate, BitDepth: v.BitDepth,
				PixelFormat: v.PixelFormat, ColorRange: v.ColorRange, ColorSpace: v.ColorSpace, ColorPrimaries: v.ColorPrimaries,
				ColorTransfer: v.ColorTransfer, FieldOrder: v.FieldOrder, Rotation: v.Rotation, Language: v.Language, Title: v.Title,
			})
			if err != nil {
				return fmt.Errorf("insert video stream failed: %w", err)
			}
			return nil
		},
		func(a scanner.AudioStreamFields) error {
			err := qtx.InsertAudioStream(ctx, database.InsertAudioStreamParams{
				MovieID: movieID, StreamIndex: a.StreamIndex, Codec: a.Codec, CodecProfile: a.CodecProfile, BitRate: a.BitRate,
				SampleRate: a.SampleRate, Channels: a.Channels, ChannelLayout: a.ChannelLayout, Language: a.Language, Title: a.Title, IsDefault: a.IsDefault,
			})
			if err != nil {
				return fmt.Errorf("insert audio stream failed: %w", err)
			}
			return nil
		},
		func(sub scanner.SubtitleFields) error {
			err := qtx.InsertSubtitle(ctx, database.InsertSubtitleParams{
				MovieID: movieID, StreamIndex: sub.StreamIndex, Codec: sub.Codec, Language: sub.Language, Title: sub.Title, IsForced: sub.IsForced, IsDefault: sub.IsDefault,
			})
			if err != nil {
				return fmt.Errorf("insert subtitle failed: %w", err)
			}
			return nil
		})
}
