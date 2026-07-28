package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"igloo/cmd/internal/helpers"

	"github.com/patrickmn/go-cache"
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

// streamFileCache is a read-through cache whose fills are ordered against the
// mutations that invalidate them. A reader that missed the cache may still be
// in its database query when a delete or a rescan evicts the key; without the
// generation guard that reader would publish the row it read before the
// mutation and keep a deleted or moved file streamable until the TTL expired.
//
// The generation is global rather than per-key: fills take microseconds and
// invalidations happen only on delete and rescan, so discarding a handful of
// unrelated in-flight fills costs nothing and keeps the counter allocation-free.
type streamFileCache struct {
	entries *cache.Cache
	gen     atomic.Uint64
}

func newStreamFileCache(ttl time.Duration, sweep time.Duration) *streamFileCache {
	return &streamFileCache{entries: cache.New(ttl, sweep)}
}

// generation must be read before the database query whose result will be
// published with setIfCurrent.
func (c *streamFileCache) generation() uint64 {
	return c.gen.Load()
}

func (c *streamFileCache) get(key string) (streamFile, bool) {
	cached, hit := c.entries.Get(key)
	if !hit {
		return streamFile{}, false
	}

	resolved, ok := cached.(streamFile)
	if !ok {
		return streamFile{}, false
	}

	return resolved, true
}

// setIfCurrent publishes a fill only when nothing was invalidated since gen was
// read. A stale fill is dropped rather than cached.
func (c *streamFileCache) setIfCurrent(key string, gen uint64, resolved streamFile) {
	if c.gen.Load() != gen {
		return
	}

	c.entries.SetDefault(key, resolved)
}

// invalidate must be called after the mutation commits, so a racing fill either
// reads the new row or is discarded by the generation bump.
func (c *streamFileCache) invalidate(key string) {
	c.gen.Add(1)
	c.entries.Delete(key)
}

// invalidateAll is for mutations that remove an unknown set of keys, such as an
// album delete cascading to its tracks.
func (c *streamFileCache) invalidateAll() {
	c.gen.Add(1)
	c.entries.Flush()
}

func movieStreamFileKey(movieID int64) string {
	return "movie:" + strconv.FormatInt(movieID, 10)
}

func trackStreamFileKey(trackID int64) string {
	return "track:" + strconv.FormatInt(trackID, 10)
}

// resolveStreamFile is the shared read-through body: movies and tracks differ
// only in their key and in the query that resolves a miss.
func (app *Application) resolveStreamFile(key string, resolve func() (streamFile, error)) (streamFile, error) {
	cached, hit := app.StreamFileCache.get(key)
	if hit {
		return cached, nil
	}

	gen := app.StreamFileCache.generation()

	resolved, err := resolve()
	if err != nil {
		return streamFile{}, err
	}

	app.StreamFileCache.setIfCurrent(key, gen, resolved)

	return resolved, nil
}

// movieStreamFile resolves the file behind a movie, caching the lookup. The
// caller still maps sql.ErrNoRows to 404.
func (app *Application) movieStreamFile(ctx context.Context, movieID int64) (streamFile, error) {
	return app.resolveStreamFile(movieStreamFileKey(movieID), func() (streamFile, error) {
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
	return app.resolveStreamFile(trackStreamFileKey(trackID), func() (streamFile, error) {
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
