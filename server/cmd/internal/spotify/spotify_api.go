package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const spotifyAPIBaseURL = "https://api.spotify.com/v1"

// StatusError retains the status of a failed Spotify API request and the
// message Spotify sent, without the request URL.
type StatusError struct {
	StatusCode int
	Message    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("spotify status %d: %s", e.StatusCode, e.Message)
}

// search runs one catalog search; searchType is "album", "artist" or "track".
func (s *spotifyClient) search(ctx context.Context, query, searchType string, limit int) (*searchResult, error) {
	params := url.Values{
		"q":     {query},
		"type":  {searchType},
		"limit": {strconv.Itoa(limit)},
	}

	var result searchResult
	err := s.getJSON(ctx, spotifyAPIBaseURL+"/search?"+params.Encode(), &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *spotifyClient) fetchAlbum(ctx context.Context, id ID) (*FullAlbum, error) {
	var album FullAlbum
	err := s.getJSON(ctx, spotifyAPIBaseURL+"/albums/"+url.PathEscape(id.String()), &album)
	if err != nil {
		return nil, err
	}

	return &album, nil
}

// getJSON makes a single authenticated GET; any status but 200 is an error.
func (s *spotifyClient) getJSON(ctx context.Context, requestURL string, result any) error {
	accessToken, err := s.tokens.token(ctx, s.httpClient)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeStatusError(resp)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// decodeStatusError reads Spotify's {"error":{"status","message"}} envelope,
// falling back to the status text when the body carries no message.
func decodeStatusError(resp *http.Response) error {
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&body)

	message := body.Error.Message
	if decodeErr != nil || message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	return &StatusError{StatusCode: resp.StatusCode, Message: message}
}
