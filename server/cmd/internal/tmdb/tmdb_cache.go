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
