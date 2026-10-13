package main

import (
	"context"
	"strconv"
	"time"

	"igloo/cmd/internal/database"
)

const (
	// The watch-room pin checks run on every direct-play byte range and every
	// HLS manifest refresh, which must not queue behind the scanner on the
	// single connection. Rescans and deletes evict; the TTL is a backstop.
	movieStreamsCacheTTL   = time.Minute
	movieStreamsCacheSweep = 2 * time.Minute
)

// movieStreams is a movie's probed track lists, as the watch-room stream-pin
// checks need them: both in one entry, because an HLS room verifies both pins on
// the same request and a miss should cost one fill, not two.
type movieStreams struct {
	Audio     []database.AudioStream
	Subtitles []database.Subtitle
}

func movieStreamsKey(movieID int64) string {
	return "movie-streams:" + strconv.FormatInt(movieID, 10)
}

// movieStreamsFor resolves a movie's audio and subtitle streams, caching them.
// Stream-ordinal drift (audit H14) is still detected: a rescan is what rewrites
// these rows, and it invalidates this cache in the same commit hook that
// invalidates streamFileCache, so the next pin check reads the new ordinals.
func (app *Application) movieStreamsFor(ctx context.Context, movieID int64) (movieStreams, error) {
	return app.MovieStreamsCache.resolve(movieStreamsKey(movieID), func() (movieStreams, error) {
		audio, err := app.Queries.GetAudioStreamsByMovieID(ctx, movieID)
		if err != nil {
			return movieStreams{}, err
		}

		subtitles, err := app.Queries.GetSubtitlesByMovieID(ctx, movieID)
		if err != nil {
			return movieStreams{}, err
		}

		return movieStreams{Audio: audio, Subtitles: subtitles}, nil
	})
}
