package spotify

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *spotifyClient) SearchAndGetAlbumDetails(query string) (*spotify.FullAlbum, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Check cache first
	if cached, exists := s.getAlbum(query); exists {
		return cached, nil
	}

	ctx := context.Background()

	results, err := s.client.Search(ctx, query, spotify.SearchTypeAlbum, spotify.Limit(1))
	if err != nil {
		return nil, fmt.Errorf("failed to search albums for query '%s': %w", query, err)
	}

	if len(results.Albums.Albums) == 0 {
		return nil, fmt.Errorf("no albums found for query '%s'", query)
	}

	albumID := results.Albums.Albums[0].ID.String()

	album, err := s.client.GetAlbum(ctx, spotify.ID(albumID))
	if err != nil {
		return nil, fmt.Errorf("failed to get album details for ID %s: %w", albumID, err)
	}

	// Store in cache
	s.setAlbum(query, album)

	return album, nil
}
