package tmdb

import (
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
)

const (
	tmdbMovieCacheTTL     = 24 * time.Hour
	tmdbMovieCacheCleanup = 30 * time.Minute
)

func (t *tmdbClient) getMovie(id int) (*TmdbMovie, bool) {
	v, found := t.movieCache.Get(fmt.Sprintf("movie:%d", id))
	if !found {
		return nil, false
	}
	return v.(*TmdbMovie), true
}

func (t *tmdbClient) setMovie(id int, movie *TmdbMovie) {
	t.movieCache.Set(fmt.Sprintf("movie:%d", id), movie, cache.DefaultExpiration)
}

func (t *tmdbClient) ClearCache() {
	t.movieCache.Flush()
}
