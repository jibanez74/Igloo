package spotify

import (
	"context"
	"sync"

	"igloo/cmd/internal/helpers"

	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2/clientcredentials"
)

type SpotifyInterface interface {
	SearchAndGetAlbumDetails(ctx context.Context, query string) (*spotify.FullAlbum, error)
	SearchArtistByName(ctx context.Context, artistName string) (*spotify.FullArtist, error)
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
		TokenURL:     "https://accounts.spotify.com/api/token",
	}

	httpClient := config.Client(ctx)
	client := spotify.New(httpClient)

	return &spotifyClient{
		client:      client,
		artistCache: make(map[string]*spotify.FullArtist),
		artistKeys:  make([]string, 0, helpers.SPOTIFY_ARTIST_MAX_CACHE),
		albumCache:  make(map[string]*spotify.FullAlbum),
		albumKeys:   make([]string, 0, helpers.SPOTIFY_ALBUM_MAX_CACHE),
	}, nil
}
