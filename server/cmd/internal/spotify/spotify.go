package spotify

import (
	"context"
	"errors"
	"fmt"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

func New(clientID, clientSecret string) (*SpotifyClient, error) {
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("spotify client requires a client id and a client secret")
	}

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

	return &SpotifyClient{client: client}, nil
}
