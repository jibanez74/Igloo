package jellyfin

import "fmt"

func (c *JellyfinClient) GetUsers() ([]JellyfinUser, error) {
	endpoint := "/Users"

	var users []JellyfinUser
	err := c.makeRequest("GET", endpoint, nil, &users)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	return users, nil
}
