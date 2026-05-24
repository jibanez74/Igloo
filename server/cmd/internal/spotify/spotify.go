package spotify

import (
	"context"
	"fmt"

	cache "github.com/patrickmn/go-cache"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2/clientcredentials"
)

type SpotifyInterface interface {
	SearchAndGetAlbumDetails(ctx context.Context, title, artist string) (*spotify.FullAlbum, error)
	SearchArtistByName(ctx context.Context, artistName string) (*spotify.FullArtist, error)
	ClearAllCaches()
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

	_, err := config.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get spotify token: %w", err)
	}

	httpClient := config.Client(ctx)
	client := spotify.New(httpClient)

	return &spotifyClient{
		client:      client,
		artistCache: cache.New(spotifyArtistCacheTTL, spotifyCacheCleanup),
		albumCache:  cache.New(spotifyAlbumCacheTTL, spotifyCacheCleanup),
	}, nil
}
