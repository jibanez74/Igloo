package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"igloo/cmd/internal/database"
)

// mediaKind names the library entity a playback request addresses. Movies and
// TV episodes share one playback pipeline (direct play, HLS, subtitles, watch
// progress); the kind only decides which rows the pipeline is fed from.
type mediaKind string

const (
	mediaKindMovie   mediaKind = "movie"
	mediaKindEpisode mediaKind = "episode"
)

// mediaRef is the client-facing playback identity: the movie id, or the
// episode id. It is what URLs carry, what HLS session keys and the direct-stream
// cache are scoped by, and what watch progress is stored against. It is never
// a file id: two episodes in one combined file are two refs.
type mediaRef struct {
	Kind mediaKind
	ID   int64
}

func movieRef(id int64) mediaRef {
	return mediaRef{Kind: mediaKindMovie, ID: id}
}

func episodeRef(id int64) mediaRef {
	return mediaRef{Kind: mediaKindEpisode, ID: id}
}

// String is the ref's cache-key and log form, e.g. "movie:12" or "episode:99".
func (ref mediaRef) String() string {
	return string(ref.Kind) + ":" + strconv.FormatInt(ref.ID, 10)
}

// notFoundMessage is the public 404 text for this kind of media.
func (ref mediaRef) notFoundMessage() string {
	return string(ref.Kind) + " not found"
}

// invalidMediaIDMessage is the public 400 text for a malformed id of this
// kind.
func invalidMediaIDMessage(kind mediaKind) string {
	return "invalid " + string(kind) + " id"
}

// playbackSource is everything the playback pipeline needs to know about the
// file behind a mediaRef. FileID is the physical-file identity the per-file
// persistence keys on (keyframe indexes, remux verdicts, the subtitle cache):
// the movie id for a movie, since a movie row is its file, and show_files.id
// for an episode, so episodes sharing a combined file share those rows.
type playbackSource struct {
	Ref       mediaRef
	FileID    int64
	FilePath  string
	FileName  string
	Container string
	MimeType  string
	Size      int64
	UpdatedAt string
	Duration  sql.NullFloat64
}

func playbackSourceFromMovie(movie database.Movie) playbackSource {
	return playbackSource{
		Ref:       movieRef(movie.ID),
		FileID:    movie.ID,
		FilePath:  movie.FilePath,
		FileName:  movie.FileName,
		Container: movie.Container,
		MimeType:  movie.MimeType,
		Size:      movie.Size,
		UpdatedAt: movie.UpdatedAt,
		Duration:  movie.Duration,
	}
}

func playbackSourceFromShowFile(episodeID int64, file database.GetShowFileForEpisodeRow) playbackSource {
	return playbackSource{
		Ref:       episodeRef(episodeID),
		FileID:    file.ID,
		FilePath:  file.FilePath,
		FileName:  file.FileName,
		Container: file.Container,
		MimeType:  file.MimeType,
		Size:      file.Size,
		UpdatedAt: file.UpdatedAt,
		Duration:  file.Duration,
	}
}

// loadPlaybackSource resolves the file behind a ref. sql.ErrNoRows is returned
// unwrapped for both kinds so callers keep one not-found check.
func (app *Application) loadPlaybackSource(ctx context.Context, ref mediaRef) (playbackSource, error) {
	switch ref.Kind {
	case mediaKindMovie:
		movie, err := app.Queries.GetMovieByID(ctx, ref.ID)
		if err != nil {
			return playbackSource{}, err
		}
		return playbackSourceFromMovie(movie), nil
	case mediaKindEpisode:
		file, err := app.Queries.GetShowFileForEpisode(ctx, ref.ID)
		if err != nil {
			return playbackSource{}, err
		}
		return playbackSourceFromShowFile(ref.ID, file), nil
	default:
		return playbackSource{}, fmt.Errorf("unknown media kind %q", ref.Kind)
	}
}

// The show stream tables are column-for-column the movie stream tables keyed
// on a file instead of a movie, and every playback decision (browser-safe
// remux gate, HDR, interlacing, audio profiles, subtitle ordinals) reads only
// the probe columns. The converters below let the show rows flow through that
// code as the movie row types; ID and MovieID are left zero because nothing on
// the playback path reads them. Chapters need no converter: they are only
// served, never used to build a session, and the technical-details handler
// serializes the show rows as they are.

func showVideoStreamAsPlayback(row database.GetShowVideoStreamsByFileIDRow) database.VideoStream {
	return database.VideoStream{
		StreamIndex:    row.StreamIndex,
		Codec:          row.Codec,
		CodecProfile:   row.CodecProfile,
		CodecLevel:     row.CodecLevel,
		BitRate:        row.BitRate,
		Width:          row.Width,
		Height:         row.Height,
		CodedWidth:     row.CodedWidth,
		CodedHeight:    row.CodedHeight,
		AspectRatio:    row.AspectRatio,
		FrameRate:      row.FrameRate,
		AvgFrameRate:   row.AvgFrameRate,
		BitDepth:       row.BitDepth,
		PixelFormat:    row.PixelFormat,
		ColorRange:     row.ColorRange,
		ColorSpace:     row.ColorSpace,
		ColorPrimaries: row.ColorPrimaries,
		ColorTransfer:  row.ColorTransfer,
		FieldOrder:     row.FieldOrder,
		Rotation:       row.Rotation,
		Language:       row.Language,
		Title:          row.Title,
	}
}

func showAudioStreamAsPlayback(row database.GetShowAudioStreamsByFileIDRow) database.AudioStream {
	return database.AudioStream{
		StreamIndex:   row.StreamIndex,
		Codec:         row.Codec,
		CodecProfile:  row.CodecProfile,
		BitRate:       row.BitRate,
		SampleRate:    row.SampleRate,
		Channels:      row.Channels,
		ChannelLayout: row.ChannelLayout,
		Language:      row.Language,
		Title:         row.Title,
		IsDefault:     row.IsDefault,
	}
}

func showSubtitleAsPlayback(row database.GetShowSubtitlesByFileIDRow) database.Subtitle {
	return database.Subtitle{
		StreamIndex: row.StreamIndex,
		Codec:       row.Codec,
		Language:    row.Language,
		Title:       row.Title,
		IsForced:    row.IsForced,
		IsDefault:   row.IsDefault,
	}
}

// loadPlaybackVideoStreams returns the source's video streams in stream_index
// order, whichever table they live in.
func loadPlaybackVideoStreams(ctx context.Context, q *database.Queries, source playbackSource) ([]database.VideoStream, error) {
	if source.Ref.Kind == mediaKindMovie {
		return q.GetVideoStreamsByMovieID(ctx, source.FileID)
	}

	rows, err := q.GetShowVideoStreamsByFileID(ctx, source.FileID)
	if err != nil {
		return nil, err
	}
	streams := make([]database.VideoStream, len(rows))
	for i, row := range rows {
		streams[i] = showVideoStreamAsPlayback(row)
	}
	return streams, nil
}

// loadPlaybackAudioStreams is the audio twin of loadPlaybackVideoStreams. The
// ordinal the client selects with audio_track is the position in this slice.
func loadPlaybackAudioStreams(ctx context.Context, q *database.Queries, source playbackSource) ([]database.AudioStream, error) {
	if source.Ref.Kind == mediaKindMovie {
		return q.GetAudioStreamsByMovieID(ctx, source.FileID)
	}

	rows, err := q.GetShowAudioStreamsByFileID(ctx, source.FileID)
	if err != nil {
		return nil, err
	}
	streams := make([]database.AudioStream, len(rows))
	for i, row := range rows {
		streams[i] = showAudioStreamAsPlayback(row)
	}
	return streams, nil
}

// loadPlaybackSubtitles is the subtitle twin. The web.vtt route's trackIndex is
// the position in this slice, not the ffprobe stream index.
func loadPlaybackSubtitles(ctx context.Context, q *database.Queries, source playbackSource) ([]database.Subtitle, error) {
	if source.Ref.Kind == mediaKindMovie {
		return q.GetSubtitlesByMovieID(ctx, source.FileID)
	}

	rows, err := q.GetShowSubtitlesByFileID(ctx, source.FileID)
	if err != nil {
		return nil, err
	}
	subtitles := make([]database.Subtitle, len(rows))
	for i, row := range rows {
		subtitles[i] = showSubtitleAsPlayback(row)
	}
	return subtitles, nil
}

// parseMediaID reads the {id} route param for a handler serving one kind of
// media; the error text names the kind so a client sees which id was bad.
func parseMediaID(raw string, kind mediaKind) (mediaRef, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return mediaRef{}, errors.New(invalidMediaIDMessage(kind))
	}
	return mediaRef{Kind: kind, ID: id}, nil
}
