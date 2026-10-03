package preroll

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/tmdb"
)

type fakeSource struct {
	mu            sync.Mutex
	listCalls     int
	detailCalls   int
	active        int
	maxActive     int
	listErr       error
	movies        []*tmdb.TmdbMovie
	details       map[int]tmdb.TmdbMovie
	detailDelay   time.Duration
	releaseDetail chan struct{}
}

func (f *fakeSource) GetMoviesInTheaters(context.Context) ([]*tmdb.TmdbMovie, error) {
	f.mu.Lock()
	f.listCalls++
	f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.movies, nil
}

func (f *fakeSource) GetTmdbMovieByID(ctx context.Context, movie *tmdb.TmdbMovie) error {
	f.mu.Lock()
	f.detailCalls++
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()

	if f.releaseDetail != nil {
		select {
		case <-f.releaseDetail:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.detailDelay > 0 {
		time.Sleep(f.detailDelay)
	}
	details, ok := f.details[movie.TmdbID]
	if !ok {
		return errors.New("details unavailable")
	}
	*movie = details
	return nil
}

func video(key, site, kind string, official bool) tmdb.TmdbVideoResult {
	return tmdb.TmdbVideoResult{ID: key, Key: key, Name: key, Site: site, Type: kind, Official: official}
}

func movieWithVideos(id int, title string, videos ...tmdb.TmdbVideoResult) tmdb.TmdbMovie {
	movie := tmdb.TmdbMovie{TmdbID: id, Title: title}
	movie.Videos.Results = videos
	return movie
}

func newPool(source TheaterSource) *TheatersPool {
	return NewTheatersPool(source, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestBestYouTubeTrailerKey(t *testing.T) {
	cases := []struct {
		name   string
		videos []tmdb.TmdbVideoResult
		want   string
		found  bool
	}{
		{"trailer beats teaser", []tmdb.TmdbVideoResult{video("teaser", "YouTube", "Teaser", true), video("trailer", "YouTube", "Trailer", false)}, "trailer", true},
		{"official trailer beats unofficial", []tmdb.TmdbVideoResult{video("fan", "YouTube", "Trailer", false), video("official", "YouTube", "Trailer", true)}, "official", true},
		{"teaser when no trailer", []tmdb.TmdbVideoResult{video("clip", "YouTube", "Clip", true), video("teaser", "YouTube", "Teaser", false)}, "teaser", true},
		{"non youtube ignored", []tmdb.TmdbVideoResult{video("v", "Vimeo", "Trailer", true)}, "", false},
		{"case insensitive site and type", []tmdb.TmdbVideoResult{video("k", "youtube", "TRAILER", false)}, "k", true},
		{"empty", nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := BestYouTubeTrailerKey(tc.videos)
			if got != tc.want || found != tc.found {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, found, tc.want, tc.found)
			}
		})
	}
}

func TestTheatersPoolResolvesAndCaches(t *testing.T) {
	source := &fakeSource{
		movies: []*tmdb.TmdbMovie{{TmdbID: 1, Title: "One"}, {TmdbID: 2, Title: "Two"}, {TmdbID: 3, Title: "No trailer"}},
		details: map[int]tmdb.TmdbMovie{
			1: movieWithVideos(1, "One", video("one-key", "YouTube", "Trailer", true)),
			2: movieWithVideos(2, "", video("two-key", "YouTube", "Teaser", false)),
			3: movieWithVideos(3, "No trailer", video("x", "Vimeo", "Trailer", true)),
		},
	}
	pool := newPool(source)

	trailers := pool.Trailers(context.Background())
	if len(trailers) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(trailers), trailers)
	}
	if trailers[0].YouTubeKey != "one-key" || trailers[0].TmdbID != 1 || trailers[0].Source != SourceTheaters {
		t.Fatalf("first = %+v", trailers[0])
	}
	if trailers[1].Title != "Two" {
		t.Fatalf("empty detail title should fall back to the listed title, got %q", trailers[1].Title)
	}

	again := pool.Trailers(context.Background())
	if len(again) != 2 || source.listCalls != 1 || source.detailCalls != 3 {
		t.Fatalf("second call hit TMDB again: list=%d details=%d", source.listCalls, source.detailCalls)
	}

	pool.Reset()
	pool.Trailers(context.Background())
	if source.listCalls != 2 {
		t.Fatalf("Reset should force a refresh, list calls = %d", source.listCalls)
	}
}

func TestTheatersPoolBoundsConcurrency(t *testing.T) {
	movies := make([]*tmdb.TmdbMovie, 0, 12)
	details := make(map[int]tmdb.TmdbMovie, 12)
	for id := 1; id <= 12; id++ {
		movies = append(movies, &tmdb.TmdbMovie{TmdbID: id, Title: "M"})
		details[id] = movieWithVideos(id, "M", video("k", "YouTube", "Trailer", true))
	}
	source := &fakeSource{movies: movies, details: details, detailDelay: 5 * time.Millisecond}
	pool := newPool(source)

	trailers := pool.Trailers(context.Background())
	if len(trailers) != 12 {
		t.Fatalf("len = %d, want 12", len(trailers))
	}
	if source.maxActive > theatersConcurrency {
		t.Fatalf("max concurrent detail calls = %d, want <= %d", source.maxActive, theatersConcurrency)
	}
}

func TestTheatersPoolListFailureIsEmptyAndRetriedAfterTheShortTTL(t *testing.T) {
	source := &fakeSource{listErr: errors.New("no movies found in theaters")}
	pool := newPool(source)
	now := time.Now()
	pool.now = func() time.Time { return now }

	trailers := pool.Trailers(context.Background())
	if len(trailers) != 0 {
		t.Fatalf("len = %d, want 0", len(trailers))
	}
	pool.Trailers(context.Background())
	if source.listCalls != 1 {
		t.Fatalf("a failure should be cached briefly, list calls = %d", source.listCalls)
	}

	now = now.Add(theatersFailureTTL + time.Second)
	pool.Trailers(context.Background())
	if source.listCalls != 2 {
		t.Fatalf("expired failure should refresh, list calls = %d", source.listCalls)
	}
}

func TestTheatersPoolCallerDeadlineReturnsStaleOrEmpty(t *testing.T) {
	release := make(chan struct{})
	source := &fakeSource{
		movies:        []*tmdb.TmdbMovie{{TmdbID: 1, Title: "One"}},
		details:       map[int]tmdb.TmdbMovie{1: movieWithVideos(1, "One", video("one", "YouTube", "Trailer", true))},
		releaseDetail: release,
	}
	pool := newPool(source)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	cold := pool.Trailers(ctx)
	if len(cold) != 0 {
		t.Fatalf("cold cache under a deadline should be empty, got %+v", cold)
	}

	close(release)
	warm := pool.Trailers(context.Background())
	if len(warm) != 1 {
		t.Fatalf("the detached refresh should have completed, got %+v", warm)
	}
	if source.listCalls != 1 {
		t.Fatalf("the waiting call should join the in-flight refresh, list calls = %d", source.listCalls)
	}

	// Expire the entry: a slow refresh now serves the stale list.
	pool.mu.Lock()
	pool.expiresAt = time.Time{}
	pool.mu.Unlock()
	source.releaseDetail = make(chan struct{})
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel2()
	stale := pool.Trailers(ctx2)
	if len(stale) != 1 {
		t.Fatalf("expired cache under a deadline should serve the stale list, got %+v", stale)
	}
	close(source.releaseDetail)
}

func TestTheatersPoolNilIsEmpty(t *testing.T) {
	var pool *TheatersPool
	if got := pool.Trailers(context.Background()); got != nil {
		t.Fatalf("nil pool = %+v, want nil", got)
	}
	if got := newPool(nil).Trailers(context.Background()); got != nil {
		t.Fatalf("nil source = %+v, want nil", got)
	}
}
