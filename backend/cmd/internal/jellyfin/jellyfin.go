package jellyfin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

func New() (*JellyfinClient, error) {
	token := os.Getenv("JELLYFIN_TOKEN")
	if token == "" {
		return nil, errors.New("jellyfin token is required to initialize client")
	}

	userID := os.Getenv("JELLYFIN_USERID")
	if userID == "" {
		return nil, errors.New("jellyfin user id is required to initialize a client")
	}

	baseUrl := os.Getenv("JELLYFIN_URL")
	if baseUrl == "" {
		return nil, errors.New("jellyfin base url is required to initialize a client")
	}

	clientName := os.Getenv("CLIENT_NAME")
	if clientName == "" {
		clientName = "igloo"
	}

	clientVersion := os.Getenv("CLIENT_VERSION")
	if clientVersion == "" {
		clientVersion = "1.0.0"
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := &JellyfinClient{
		Token:         token,
		BaseUrl:       baseUrl,
		ClientName:    clientName,
		ClientVersion: clientVersion,
		UserID:        userID,
		httpClient:    httpClient,
	}

	return client, nil
}

func (c *JellyfinClient) makeRequest(method, endpoint string, body interface{}, result interface{}) error {
	url := c.BaseUrl + endpoint

	var req *http.Request
	var err error

	if body != nil && (method == "POST" || method == "PUT" || method == "PATCH") {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
	} else {
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return err
		}
	}

	req.Header.Set("X-Emby-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s", c.ClientName, c.ClientVersion))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}
