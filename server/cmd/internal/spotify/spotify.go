package spotify

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cache "github.com/patrickmn/go-cache"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const spotifyHTTPTimeout = 15 * time.Second

type SpotifyInterface interface {
	SearchAndGetAlbumDetails(ctx context.Context, input AlbumSearchInput) (*spotify.FullAlbum, error)
	SearchArtistByName(ctx context.Context, artistName string) (*spotify.FullArtist, error)
	ClearAllCaches()
}

type AlbumSearchInput struct {
	Title       string
	Artist      string
	Year        int
	TrackTitles []string
}

var _ SpotifyInterface = (*spotifyClient)(nil)

type spotifyClient struct {
	client      *spotify.Client
	artistCache *cache.Cache
	albumCache  *cache.Cache
}

func New(ctx context.Context, clientID, clientSecret string) (SpotifyInterface, error) {
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     "https://accounts.spotify.com/api/token",
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Value(oauth2.HTTPClient) == nil {
		httpClient := &http.Client{Timeout: spotifyHTTPTimeout}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	}

	_, err := config.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get spotify token: %w", err)
	}

	client := spotify.New(config.Client(ctx), spotify.WithRetry(true))

	return &spotifyClient{
		client:      client,
		artistCache: cache.New(spotifyArtistCacheTTL, spotifyCacheCleanup),
		albumCache:  cache.New(spotifyAlbumCacheTTL, spotifyCacheCleanup),
	}, nil
}

func spotifyRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, spotifyHTTPTimeout)
}
