package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
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

// A scan stores a response for every title it resolves and reads almost none
// of them again, so the cache must not grow with the library.
func TestCacheStaysBoundedThroughAScan(t *testing.T) {
	var calls atomic.Int32
	client := newServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"id":70,"season_number":1,"episodes":[{"id":71,"season_number":1,"episode_number":1}],"aggregate_credits":{},"videos":{"results":[]}}`)
	}))

	for id := 1; id <= tmdbCacheMaxEntries*3; id++ {
		client.setMovie(id, &TmdbMovie{TmdbID: id})

		count := client.movieCache.ItemCount()
		if count > tmdbCacheMaxEntries {
			t.Fatalf("cache holds %d responses after %d movies, want at most %d", count, id, tmdbCacheMaxEntries)
		}
	}

	_, found := client.getMovie(tmdbCacheMaxEntries * 3)
	if !found {
		t.Error("the response stored last was not kept")
	}

	// The cache is full again here, so the first season request empties it;
	// the episodes of that season must still share the one response.
	for len(client.movieCache.Items()) < tmdbCacheMaxEntries {
		client.setMovie(-len(client.movieCache.Items())-1, &TmdbMovie{})
	}
	for episode := 0; episode < 3; episode++ {
		_, err := client.GetSeasonDetails(context.Background(), 7, 1)
		if err != nil {
			t.Fatalf("season request %d: %v", episode, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("season requests reached TMDB %d times, want 1", calls.Load())
	}
}
