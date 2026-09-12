package show

import (
	"context"
	"fmt"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
)

func processStreams(
	ctx context.Context,
	qtx *database.Queries,
	fileID int64,
	streams []ffprobe.Stream,
) (videoStreamCount int, err error) {
	err = qtx.DeleteShowFileVideoStreams(ctx, fileID)
	if err != nil {
		return 0, fmt.Errorf("delete show video streams failed: %w", err)
	}
	err = qtx.DeleteShowFileAudioStreams(ctx, fileID)
	if err != nil {
		return 0, fmt.Errorf("delete show audio streams failed: %w", err)
	}
	err = qtx.DeleteShowFileSubtitles(ctx, fileID)
	if err != nil {
		return 0, fmt.Errorf("delete show subtitles failed: %w", err)
	}

	return scanner.ClassifyStreams(streams,
		func(v scanner.VideoStreamFields) error {
			_, err := qtx.InsertShowVideoStream(ctx, database.InsertShowVideoStreamParams{
				FileID: fileID, StreamIndex: v.StreamIndex, Codec: v.Codec, CodecProfile: v.CodecProfile, CodecLevel: v.CodecLevel,
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
			_, err := qtx.InsertShowAudioStream(ctx, database.InsertShowAudioStreamParams{
				FileID: fileID, StreamIndex: a.StreamIndex, Codec: a.Codec, CodecProfile: a.CodecProfile, BitRate: a.BitRate,
				SampleRate: a.SampleRate, Channels: a.Channels, ChannelLayout: a.ChannelLayout, Language: a.Language, Title: a.Title, IsDefault: a.IsDefault,
			})
			if err != nil {
				return fmt.Errorf("insert audio stream failed: %w", err)
			}
			return nil
		},
		func(sub scanner.SubtitleFields) error {
			_, err := qtx.InsertShowSubtitle(ctx, database.InsertShowSubtitleParams{
				FileID: fileID, StreamIndex: sub.StreamIndex, Codec: sub.Codec, Language: sub.Language, Title: sub.Title, IsForced: sub.IsForced, IsDefault: sub.IsDefault,
			})
			if err != nil {
				return fmt.Errorf("insert subtitle failed: %w", err)
			}
			return nil
		})
}
