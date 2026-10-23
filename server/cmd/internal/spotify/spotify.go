package spotify

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const spotifyHTTPTimeout = 15 * time.Second

// Client talks to the Spotify Web API with an app token. The music scanner
// and the request-picker endpoints each declare the subset of its methods
// they call.
type Client struct {
	httpClient *http.Client
	tokens     *tokenSource
}

// New fetches the first app token up front, so bad credentials fail at startup
// rather than on the first lookup.
func New(ctx context.Context, clientID, clientSecret string) (*Client, error) {
	return newClient(ctx, &http.Client{}, clientID, clientSecret)
}

// newClient leaves httpClient without a Timeout: every request runs under
// spotifyRequestContext, which keeps a caller's own deadline.
func newClient(ctx context.Context, httpClient *http.Client, clientID, clientSecret string) (*Client, error) {
	client := &Client{
		httpClient: httpClient,
		tokens:     &tokenSource{clientID: clientID, clientSecret: clientSecret},
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
