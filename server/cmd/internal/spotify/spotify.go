package spotify

import (
	"context"
	"fmt"
	"sync"

	"igloo/cmd/internal/helpers"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

type SpotifyInterface interface {
	SearchAndGetAlbumDetails(query string) (*spotify.FullAlbum, error)
	SearchArtistByName(artistName string) (*spotify.FullArtist, error)
	ClearAllCaches()
}

// Compile-time check to ensure spotifyClient implements SpotifyInterface
var _ SpotifyInterface = (*spotifyClient)(nil)

type spotifyClient struct {
	client      *spotify.Client
	artistCache map[string]*spotify.FullArtist
	artistKeys  []string
	artistMu    sync.RWMutex
	albumCache  map[string]*spotify.FullAlbum
	albumKeys   []string
	albumMu     sync.RWMutex
}

func New(clientID, clientSecret string) (SpotifyInterface, error) {
	ctx := context.Background()

	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     spotifyauth.TokenURL,
	}

	token, err := config.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)

	return &spotifyClient{
		client:      client,
		artistCache: make(map[string]*spotify.FullArtist),
		artistKeys:  make([]string, 0, helpers.SPOTIFY_ARTIST_MAX_CACHE),
		albumCache:  make(map[string]*spotify.FullAlbum),
		albumKeys:   make([]string, 0, helpers.SPOTIFY_ALBUM_MAX_CACHE),
	}, nil
}
