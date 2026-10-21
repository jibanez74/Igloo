package spotify

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cache "github.com/patrickmn/go-cache"
)

const spotifyHTTPTimeout = 15 * time.Second

type SpotifyInterface interface {
	SearchAndGetAlbumDetails(ctx context.Context, title, artist string) (*FullAlbum, error)
	SearchAlbums(ctx context.Context, title string) ([]SimpleAlbum, error)
	SearchTracks(ctx context.Context, title string) ([]FullTrack, error)
	SearchArtistByName(ctx context.Context, artistName string) (*FullArtist, error)
	ClearAllCaches()
}

type spotifyClient struct {
	httpClient  *http.Client
	tokens      *tokenSource
	artistCache *cache.Cache
	albumCache  *cache.Cache
}

// New fetches the first app token up front, so bad credentials fail at startup
// rather than on the first lookup.
func New(ctx context.Context, clientID, clientSecret string) (SpotifyInterface, error) {
	client, err := newClient(ctx, &http.Client{}, clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// newClient leaves httpClient without a Timeout: every request runs under
// spotifyRequestContext, which keeps a caller's own deadline.
func newClient(ctx context.Context, httpClient *http.Client, clientID, clientSecret string) (*spotifyClient, error) {
	client := &spotifyClient{
		httpClient:  httpClient,
		tokens:      &tokenSource{clientID: clientID, clientSecret: clientSecret},
		artistCache: cache.New(spotifyArtistCacheTTL, spotifyCacheCleanup),
		albumCache:  cache.New(spotifyAlbumCacheTTL, spotifyCacheCleanup),
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	_, err := client.tokens.token(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get spotify token: %w", err)
	}

	return client, nil
}

func spotifyRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, spotifyHTTPTimeout)
}
