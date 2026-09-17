package scanner

import (
	"database/sql"
	"strconv"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
)

// VideoStreamFields, AudioStreamFields, and SubtitleFields carry one probed
// stream with every value converted to its column type. The movie and TV
// catalogs store the same technical columns in separate tables, so each
// scanner maps these onto its own sqlc params.
type VideoStreamFields struct {
	StreamIndex    int64
	Codec          string
	CodecProfile   sql.NullString
	CodecLevel     sql.NullInt64
	BitRate        int64
	Width          int64
	Height         int64
	CodedWidth     sql.NullInt64
	CodedHeight    sql.NullInt64
	AspectRatio    sql.NullString
	FrameRate      float64
	AvgFrameRate   sql.NullString
	BitDepth       sql.NullInt64
	PixelFormat    sql.NullString
	ColorRange     sql.NullString
	ColorSpace     sql.NullString
	ColorPrimaries sql.NullString
	ColorTransfer  sql.NullString
	FieldOrder     sql.NullString
	Rotation       sql.NullInt64
	Language       sql.NullString
	Title          sql.NullString
}

type AudioStreamFields struct {
	StreamIndex   int64
	Codec         string
	CodecProfile  sql.NullString
	BitRate       int64
	SampleRate    sql.NullInt64
	Channels      int64
	ChannelLayout sql.NullString
	Language      sql.NullString
	Title         sql.NullString
	IsDefault     bool
}

type SubtitleFields struct {
	StreamIndex int64
	Codec       string
	Language    sql.NullString
	Title       sql.NullString
	IsForced    bool
	IsDefault   bool
}

// ClassifyStreams routes each probed stream to the matching callback and
// returns how many video streams were accepted. Attached pictures and
// cover-art codecs are skipped: they are artwork, not playable video.
func ClassifyStreams(
	streams []ffprobe.Stream,
	onVideo func(VideoStreamFields) error,
	onAudio func(AudioStreamFields) error,
	onSubtitle func(SubtitleFields) error,
) (int, error) {
	videoCount := 0
	for _, stream := range streams {
		var err error
		switch stream.CodecType {
		case "video":
			artwork := stream.Disposition.AttachedPic == 1 || helpers.IsCoverArtVideoCodec(stream.CodecName)
			if artwork {
				continue
			}
			err = onVideo(videoStreamFields(stream))
			if err == nil {
				videoCount++
			}
		case "audio":
			err = onAudio(audioStreamFields(stream))
		case "subtitle":
			err = onSubtitle(subtitleFields(stream))
		}
		if err != nil {
			return 0, err
		}
	}
	return videoCount, nil
}

func videoStreamFields(stream ffprobe.Stream) VideoStreamFields {
	// An explicit 0-degree display matrix persists as 0 while absence persists
	// as NULL, so helpers.NullInt64 (which maps 0 to NULL) does not fit here.
	var rotation sql.NullInt64
	rotationDeg, hasRotation := stream.Rotation()
	if hasRotation {
		rotation = sql.NullInt64{Int64: rotationDeg, Valid: true}
	}
	return VideoStreamFields{
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
	}
}

func audioStreamFields(stream ffprobe.Stream) AudioStreamFields {
	return AudioStreamFields{
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
	}
}

func subtitleFields(stream ffprobe.Stream) SubtitleFields {
	return SubtitleFields{
		StreamIndex: int64(stream.Index),
		Codec:       stream.CodecName,
		Language:    helpers.NullString(stream.Tags.Language),
		Title:       helpers.NullString(stream.Tags.Title),
		IsForced:    stream.Disposition.Forced == 1,
		IsDefault:   stream.Disposition.Default == 1,
	}
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
