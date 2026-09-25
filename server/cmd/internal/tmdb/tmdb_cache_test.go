package tmdb

import (
	"testing"

	cache "github.com/patrickmn/go-cache"
)

func TestGetMovieTreatsWrongTypeEntryAsMiss(t *testing.T) {
	client := newTestClient("")

	client.movieCache.Set(movieCacheKey(603), "not a movie", cache.DefaultExpiration)
	movie, found := client.getMovie(603)
	if found || movie != nil {
		t.Fatalf("getMovie = (%v, %v), want a miss for a wrong-type entry", movie, found)
	}

	client.setMovie(603, &TmdbMovie{TmdbID: 603, Title: "The Matrix"})
	movie, found = client.getMovie(603)
	if !found || movie == nil || movie.Title != "The Matrix" {
		t.Fatalf("getMovie = (%v, %v), want the stored movie", movie, found)
	}
}
