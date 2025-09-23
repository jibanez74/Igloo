package spotify

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

func New(clientID, clientSecret string) (*SpotifyClient, error) {
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

	return &SpotifyClient{
		client:      client,
		artistCache: make(map[string]*spotify.FullArtist),
		albumCache:  make(map[string]*spotify.FullAlbum),
	}, nil
}
