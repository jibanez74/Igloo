package jellyfin

import (
	"fmt"
	"net/url"
)

func (c *JellyfinClient) GetLibraries() ([]JellyfinLibrary, error) {
	endpoint := "/Library/MediaFolders"

	var libraries []JellyfinLibrary
	err := c.makeRequest("GET", endpoint, nil, &libraries)
	if err != nil {
		return nil, fmt.Errorf("failed to get libraries: %w", err)
	}

	return libraries, nil
}

func (c *JellyfinClient) GetItems(parentId string, limit int) ([]JellyfinItem, error) {
	endpoint := fmt.Sprintf("/Users/%s/Items", c.UserID)

	params := url.Values{}
	if parentId != "" {
		params.Set("ParentId", parentId)
	}
	if limit > 0 {
		params.Set("Limit", fmt.Sprintf("%d", limit))
	}

	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	var result SearchResult
	err := c.makeRequest("GET", endpoint, nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}

	return result.Items, nil
}

func (c *JellyfinClient) SearchItems(query string, limit int) ([]JellyfinItem, error) {
	endpoint := fmt.Sprintf("/Users/%s/Items", c.UserID)

	params := url.Values{}
	params.Set("SearchTerm", query)
	if limit > 0 {
		params.Set("Limit", fmt.Sprintf("%d", limit))
	}

	endpoint += "?" + params.Encode()

	var result SearchResult
	err := c.makeRequest("GET", endpoint, nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to search items: %w", err)
	}

	return result.Items, nil
}

func (c *JellyfinClient) GetItem(itemId string) (*JellyfinItem, error) {
	endpoint := fmt.Sprintf("/Users/%s/Items/%s", c.UserID, itemId)

	var item JellyfinItem
	err := c.makeRequest("GET", endpoint, nil, &item)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return &item, nil
}
