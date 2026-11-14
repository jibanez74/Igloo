package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"igloo/cmd/internal/helpers"
)

const (
	spotifyAPIBaseURL = "https://api.spotify.com/v1"
	// quotaExceededReason marks a 429 sent once the developer account's shared
	// request quota is spent; no wait inside a scan brings it back.
	quotaExceededReason = "QUOTA_EXCEEDED"
	// spotifyRateLimitRetries bounds the retries of one request after a rate
	// limit, and spotifyRateLimitMaxWait the Retry-After worth waiting for.
	spotifyRateLimitRetries = 2
	spotifyRateLimitMaxWait = 5 * time.Second
)

// StatusError retains the status of a failed Spotify API request and the
// message Spotify sent, without the request URL. Reason carries Spotify's
// machine-readable reason when the body names one.
type StatusError struct {
	StatusCode int
	Message    string
	Reason     string
}

func (e *StatusError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("spotify status %d: %s (%s)", e.StatusCode, e.Message, e.Reason)
	}
	return fmt.Sprintf("spotify status %d: %s", e.StatusCode, e.Message)
}

// ProviderFailure classifies a Spotify error for scan dispatch. A fatal
// failure (rejected credentials, exhausted quota) means no further request
// can succeed this run; a transient one (rate limit, server error, transport)
// may clear. A definitive non-match, and a status such as 404 that concerns
// one catalog entry, is neither.
func ProviderFailure(err error) (fatal, transient bool) {
	if err == nil {
		return false, false
	}

	var status *StatusError
	isStatus := errors.As(err, &status)
	if isStatus {
		return classifySpotifyStatus(status.StatusCode, status.Reason, false)
	}

	var token *TokenError
	isToken := errors.As(err, &token)
	if isToken {
		return classifySpotifyStatus(token.StatusCode, "", true)
	}

	var match *MatchError
	isMatch := errors.As(err, &match)
	noMatch := isMatch && match.Err == nil
	if noMatch {
		return false, false
	}

	return false, true
}

// classifySpotifyStatus reads a response status: the token endpoint answers
// rejected credentials with 400, the API with 401 or 403.
func classifySpotifyStatus(statusCode int, reason string, fromTokenEndpoint bool) (fatal, transient bool) {
	rejectedCredentials := statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden ||
		(fromTokenEndpoint && statusCode == http.StatusBadRequest)
	quotaExhausted := statusCode == http.StatusTooManyRequests && reason == quotaExceededReason
	if rejectedCredentials || quotaExhausted {
		return true, false
	}

	transient = statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
	return false, transient
}

// search runs one catalog search; searchType is "album", "artist" or "track".
func (c *Client) search(ctx context.Context, query, searchType string, limit int) (*searchResult, error) {
	params := url.Values{
		"q":     {query},
		"type":  {searchType},
		"limit": {strconv.Itoa(limit)},
	}

	var result searchResult
	err := c.getJSON(ctx, spotifyAPIBaseURL+"/search?"+params.Encode(), &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// getJSON makes an authenticated GET; any status but 200 is an error. A rate
// limit (429 without the quota reason) that names a short Retry-After is
// waited out and retried, up to spotifyRateLimitRetries times; a longer wait,
// or one past the request's deadline, returns the 429 for the scan's breaker.
func (c *Client) getJSON(ctx context.Context, requestURL string, result any) error {
	for attempt := 0; ; attempt++ {
		retryAfter, err := c.getJSONOnce(ctx, requestURL, result)
		if retryAfter < 0 || attempt == spotifyRateLimitRetries {
			return err
		}

		deadline, hasDeadline := ctx.Deadline()
		pastDeadline := hasDeadline && time.Now().Add(retryAfter).After(deadline)
		if pastDeadline {
			return err
		}

		sleepErr := helpers.SleepContext(ctx, retryAfter)
		if sleepErr != nil {
			return err
		}
	}
}

// getJSONOnce makes one request. retryAfter is the wait a retryable rate limit
// asks for, and negative when the request must not be retried.
func (c *Client) getJSONOnce(ctx context.Context, requestURL string, result any) (retryAfter time.Duration, err error) {
	accessToken, err := c.tokens.token(ctx, c.httpClient)
	if err != nil {
		return -1, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return -1, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return -1, json.NewDecoder(resp.Body).Decode(result)
	}

	statusErr := decodeStatusError(resp)
	retryAfter = rateLimitRetryAfter(resp, statusErr)
	return retryAfter, statusErr
}

// rateLimitRetryAfter returns the wait a 429 asks for when it is worth waiting
// inside a request, and -1 otherwise: the quota reason never clears within a
// scan, and a missing or long Retry-After is left to the scan's breaker.
func rateLimitRetryAfter(resp *http.Response, err error) time.Duration {
	if resp.StatusCode != http.StatusTooManyRequests {
		return -1
	}
	status, ok := errors.AsType[*StatusError](err)
	if !ok || status.Reason == quotaExceededReason {
		return -1
	}
	retryAfter, ok := helpers.RetryAfter(resp.Header, time.Now())
	if !ok || retryAfter > spotifyRateLimitMaxWait {
		return -1
	}
	return retryAfter
}

// decodeStatusError reads Spotify's {"error":{"status","message"}} envelope,
// falling back to the status text when the body carries no message. The quota
// reason is accepted both inside the envelope and at the top level, since
// Spotify's announcement does not say where it sits.
func decodeStatusError(resp *http.Response) error {
	var body struct {
		Reason string `json:"reason"`
		Error  struct {
			Message string `json:"message"`
			Reason  string `json:"reason"`
		} `json:"error"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&body)

	message := body.Error.Message
	if decodeErr != nil || message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	reason := body.Error.Reason
	if reason == "" {
		reason = body.Reason
	}

	return &StatusError{StatusCode: resp.StatusCode, Message: message, Reason: reason}
}
