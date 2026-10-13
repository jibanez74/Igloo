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
	v, _ := t.movieCache.Get(movieCacheKey(id))
	movie, ok := v.(*TmdbMovie)
	return movie, ok
}

func (t *tmdbClient) setMovie(id int, movie *TmdbMovie) {
	t.movieCache.Set(movieCacheKey(id), movie, cache.DefaultExpiration)
}

func (t *tmdbClient) ClearCache() {
	t.movieCache.Flush()
}
