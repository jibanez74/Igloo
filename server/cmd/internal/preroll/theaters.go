package preroll

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/tmdb"
)

const (
	theatersSuccessTTL     = 3 * time.Hour
	theatersFailureTTL     = 10 * time.Minute
	theatersRefreshTimeout = 20 * time.Second
	theatersConcurrency    = 4
)

// TheaterSource is the slice of the TMDB client the pool needs; the real
// client and the test stub both satisfy it.
type TheaterSource interface {
	GetMoviesInTheaters(ctx context.Context) ([]*tmdb.TmdbMovie, error)
	GetTmdbMovieByID(ctx context.Context, movie *tmdb.TmdbMovie) error
}

// TheatersPool keeps the resolved now-playing trailers (titles, TMDB ids and
// YouTube keys only) in memory for a few hours, so a play never fans out into
// one TMDB call per movie. Nothing is persisted.
type TheatersPool struct {
	source      TheaterSource
	logger      logger.LoggerInterface
	now         func() time.Time
	concurrency int

	mu         sync.Mutex
	trailers   []Trailer
	expiresAt  time.Time
	refreshing chan struct{}
}

func NewTheatersPool(source TheaterSource, log logger.LoggerInterface) *TheatersPool {
	return &TheatersPool{
		source:      source,
		logger:      log,
		now:         time.Now,
		concurrency: theatersConcurrency,
	}
}

// Trailers returns the cached list, refreshing it when it has expired. A
// refresh runs detached from the caller, on its own timeout, so it completes
// and caches even when the request that triggered it stops waiting; a caller
// whose ctx ends first gets the stale list, or nothing on a cold cache. A nil
// pool or source (TMDB not configured) always yields an empty pool.
func (p *TheatersPool) Trailers(ctx context.Context) []Trailer {
	if p == nil || p.source == nil {
		return nil
	}

	p.mu.Lock()
	fresh := p.now().Before(p.expiresAt)
	if fresh {
		cached := cloneTrailers(p.trailers)
		p.mu.Unlock()
		return cached
	}
	stale := cloneTrailers(p.trailers)
	done := p.refreshing
	if done == nil {
		done = make(chan struct{})
		p.refreshing = done
		go p.refresh(done)
	}
	p.mu.Unlock()

	select {
	case <-done:
		p.mu.Lock()
		resolved := cloneTrailers(p.trailers)
		p.mu.Unlock()
		return resolved
	case <-ctx.Done():
		return stale
	}
}

// Reset drops the cached list; the next request resolves it again.
func (p *TheatersPool) Reset() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.trailers = nil
	p.expiresAt = time.Time{}
	p.mu.Unlock()
}

func (p *TheatersPool) refresh(done chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), theatersRefreshTimeout)
	defer cancel()

	trailers, err := p.resolve(ctx)

	p.mu.Lock()
	ttl := theatersSuccessTTL
	if err != nil {
		p.logger.Warn("failed to resolve in-theaters trailers", "error", err)
		ttl = theatersFailureTTL
	} else {
		p.trailers = trailers
		if len(trailers) == 0 {
			ttl = theatersFailureTTL
		}
	}
	p.expiresAt = p.now().Add(ttl)
	p.refreshing = nil
	close(done)
	p.mu.Unlock()
}

func (p *TheatersPool) resolve(ctx context.Context) ([]Trailer, error) {
	movies, err := p.source.GetMoviesInTheaters(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]*Trailer, len(movies))
	permits := make(chan struct{}, p.concurrency)
	var wait sync.WaitGroup
	for index, listed := range movies {
		if listed == nil || listed.TmdbID == 0 {
			continue
		}
		permits <- struct{}{}
		wait.Add(1)
		go func(index int, tmdbID int, title string) {
			defer wait.Done()
			defer func() { <-permits }()
			results[index] = p.resolveMovie(ctx, tmdbID, title)
		}(index, listed.TmdbID, listed.Title)
	}
	wait.Wait()

	trailers := make([]Trailer, 0, len(movies))
	for _, trailer := range results {
		if trailer != nil {
			trailers = append(trailers, *trailer)
		}
	}
	ctxErr := ctx.Err()
	if ctxErr != nil && len(trailers) == 0 {
		return nil, ctxErr
	}
	return trailers, nil
}

func (p *TheatersPool) resolveMovie(ctx context.Context, tmdbID int, listedTitle string) *Trailer {
	movie := tmdb.TmdbMovie{TmdbID: tmdbID}
	err := p.source.GetTmdbMovieByID(ctx, &movie)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			p.logger.Debug("in-theaters trailer lookup failed", "tmdb_id", tmdbID, "error", err)
		}
		return nil
	}

	key, found := BestYouTubeTrailerKey(movie.Videos.Results)
	if !found {
		return nil
	}
	title := strings.TrimSpace(movie.Title)
	if title == "" {
		title = listedTitle
	}
	return &Trailer{
		Title:      title,
		YouTubeKey: key,
		Source:     SourceTheaters,
		TmdbID:     int64(tmdbID),
	}
}

// BestYouTubeTrailerKey picks a movie's YouTube video: a Trailer beats a
// Teaser, official beats unofficial, and other kinds are never used.
func BestYouTubeTrailerKey(videos []tmdb.TmdbVideoResult) (string, bool) {
	bestKey := ""
	bestScore := 0
	for _, video := range videos {
		isYouTube := strings.EqualFold(video.Site, "YouTube") && video.Key != ""
		if !isYouTube {
			continue
		}
		score := 0
		switch strings.ToLower(video.Type) {
		case "trailer":
			score = 4
		case "teaser":
			score = 2
		default:
			continue
		}
		if video.Official {
			score++
		}
		if score > bestScore {
			bestScore = score
			bestKey = video.Key
		}
	}
	return bestKey, bestKey != ""
}

func cloneTrailers(trailers []Trailer) []Trailer {
	if len(trailers) == 0 {
		return nil
	}
	cloned := make([]Trailer, len(trailers))
	copy(cloned, trailers)
	return cloned
}
