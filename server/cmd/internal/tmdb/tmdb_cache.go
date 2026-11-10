package tmdb

import (
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
)

const (
	tmdbMovieCacheTTL     = 15 * time.Minute
	tmdbMovieCacheCleanup = 5 * time.Minute
	// The home page asks for the now-playing list on every load, and TMDB
	// changes it weekly, so it is kept longer than a movie's details.
	tmdbInTheatersCacheTTL = time.Hour
	inTheatersCacheKey     = "movies:in-theaters"
	// A scan asks for each title once, so an unbounded cache holds every
	// response of a first scan's last fifteen minutes and answers almost none
	// of them again. The seasons of the show being scanned are asked for
	// close together, and still hit after a full cache is emptied.
	tmdbCacheMaxEntries = 64
)

func movieCacheKey(id int) string {
	return fmt.Sprintf("movie:%d", id)
}

func (t *tmdbClient) getMovie(id int) (*TmdbMovie, bool) {
	v, _ := t.movieCache.Get(movieCacheKey(id))
	movie, ok := v.(*TmdbMovie)
	return movie, ok
}

func (t *tmdbClient) setMovie(id int, movie *TmdbMovie) {
	t.cacheSet(movieCacheKey(id), movie, cache.DefaultExpiration)
}

// cacheSet keeps a response, emptying the cache first when it is full.
func (t *tmdbClient) cacheSet(key string, value any, ttl time.Duration) {
	isFull := t.movieCache.ItemCount() >= tmdbCacheMaxEntries
	if isFull {
		t.movieCache.Flush()
	}
	t.movieCache.Set(key, value, ttl)
}

func (t *tmdbClient) ClearCache() {
	t.movieCache.Flush()
}
