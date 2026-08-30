package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"igloo/cmd/internal/helpers"
)

const (
	// A player issues one range request per seek and one per buffer refill, and
	// the database runs on a single shared connection (InitDB), so resolving
	// the file on every one of them puts playback behind the scanner. Movie
	// deletion and rescans evict explicitly; the TTL is the backstop for every
	// other way a row can change.
	streamFileCacheTTL   = time.Minute
	streamFileCacheSweep = 2 * time.Minute
)

// streamFile is everything serveMediaFile needs to deliver a library file.
type streamFile struct {
	Path        string
	Name        string
	ContentType string
}

func movieStreamFileKey(movieID int64) string {
	return "movie:" + strconv.FormatInt(movieID, 10)
}

func trackStreamFileKey(trackID int64) string {
	return "track:" + strconv.FormatInt(trackID, 10)
}

// movieStreamFile resolves the file behind a movie, caching the lookup. The
// caller still maps sql.ErrNoRows to 404.
func (app *Application) movieStreamFile(ctx context.Context, movieID int64) (streamFile, error) {
	return app.StreamFileCache.resolve(movieStreamFileKey(movieID), func() (streamFile, error) {
		movie, err := app.Queries.GetMovieForDirectStream(ctx, movieID)
		if err != nil {
			return streamFile{}, err
		}

		return streamFile{
			Path:        movie.FilePath,
			Name:        movie.FileName,
			ContentType: movieContentType(movie.Container, movie.MimeType),
		}, nil
	})
}

// trackStreamFile is the music twin of movieStreamFile.
func (app *Application) trackStreamFile(ctx context.Context, trackID int64) (streamFile, error) {
	return app.StreamFileCache.resolve(trackStreamFileKey(trackID), func() (streamFile, error) {
		track, err := app.Queries.GetTrack(ctx, trackID)
		if err != nil {
			return streamFile{}, err
		}

		return streamFile{
			Path:        track.FilePath,
			Name:        track.FileName,
			ContentType: track.MimeType,
		}, nil
	})
}

// strongFileETag derives a strong validator from what ServeContent already
// stats: file size and nanosecond mtime. Setting it before ServeContent
// enables If-None-Match and byte-exact If-Range validation, which
// Last-Modified alone cannot provide within its one-second granularity
// (audit D5).
func strongFileETag(stat os.FileInfo) string {
	return fmt.Sprintf("\"%x-%x\"", stat.Size(), stat.ModTime().UnixNano())
}

// serveMediaFile streams a library file with range support, owning the file
// handle, the strong validator and the error response. It returns the failure
// so the caller can log it with its own context; a nil return means the body
// was served.
func serveMediaFile(w http.ResponseWriter, r *http.Request, path string, name string, contentType string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			helpers.ErrorJSON(w, errors.New("media file not found"), http.StatusNotFound)
			return err
		}

		helpers.ErrorJSON(w, errors.New("failed to open media file"))
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		helpers.ErrorJSON(w, errors.New("failed to read media file"))
		return err
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", strongFileETag(stat))

	http.ServeContent(w, r, name, stat.ModTime(), file)

	return nil
}
