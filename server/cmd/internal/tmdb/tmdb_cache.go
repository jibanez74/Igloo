package tmdb

import (
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
)

const (
	tmdbMovieCacheTTL     = 15 * time.Minute
	tmdbMovieCacheCleanup = 5 * time.Minute
)

func movieCacheKey(id int) string {
	return fmt.Sprintf("movie:%d", id)
}

func (t *tmdbClient) getMovie(id int) (*TmdbMovie, bool) {
	v, found := t.movieCache.Get(movieCacheKey(id))
	if !found {
		return nil, false
	}
	val, ok := v.(*TmdbMovie)
	if !ok {
		return nil, false
	}
	return val, true
}

func (t *tmdbClient) setMovie(id int, movie *TmdbMovie) {
	t.movieCache.Set(movieCacheKey(id), movie, cache.DefaultExpiration)
}

func (t *tmdbClient) ClearCache() {
	t.movieCache.Flush()
}
