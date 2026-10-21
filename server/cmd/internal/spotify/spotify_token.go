package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	spotifyTokenURL = "https://accounts.spotify.com/api/token"
	// A token this close to expiring is replaced, so a request never carries
	// one that lapses in flight.
	spotifyTokenExpiryMargin = time.Minute
)

// tokenSource holds the app token from Spotify's client-credentials flow. The
// mutex is held across a fetch, so concurrent callers share one refresh.
type tokenSource struct {
	clientID     string
	clientSecret string

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

func (t *tokenSource) token(ctx context.Context, httpClient *http.Client) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	fresh := t.accessToken != "" && time.Now().Add(spotifyTokenExpiryMargin).Before(t.expiry)
	if fresh {
		return t.accessToken, nil
	}

	accessToken, expiresIn, err := t.fetch(ctx, httpClient)
	if err != nil {
		return "", err
	}

	t.accessToken = accessToken
	t.expiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return t.accessToken, nil
}

func (t *tokenSource) fetch(ctx context.Context, httpClient *http.Client) (string, int, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// RFC 6749 form-encodes the client credentials before Basic encoding.
	req.SetBasicAuth(url.QueryEscape(t.clientID), url.QueryEscape(t.clientSecret))

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("spotify token request failed: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&body)

	isSuccess := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	if !isSuccess {
		reason := http.StatusText(resp.StatusCode)
		if body.Error != "" {
			reason = body.Error
		}
		if body.ErrorDescription != "" {
			reason += ": " + body.ErrorDescription
		}
		return "", 0, fmt.Errorf("spotify token request failed: status %d: %s", resp.StatusCode, reason)
	}
	if decodeErr != nil {
		return "", 0, fmt.Errorf("spotify token response: %w", decodeErr)
	}
	if body.AccessToken == "" {
		return "", 0, errors.New("spotify token response is missing access_token")
	}

	return body.AccessToken, body.ExpiresIn, nil
}
