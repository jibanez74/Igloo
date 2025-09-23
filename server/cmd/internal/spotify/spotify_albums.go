package spotify

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *SpotifyClient) SearchAndGetAlbumDetails(query string) (*spotify.FullAlbum, error) {
	ctx := context.Background()

	results, err := s.client.Search(ctx, query, spotify.SearchTypeAlbum, spotify.Limit(1))
	if err != nil {
		return nil, fmt.Errorf("failed to search albums for query '%s': %w", query, err)
	}

	if len(results.Albums.Albums) == 0 {
		return nil, fmt.Errorf("no albums found for query '%s'", query)
	}

	albumID := results.Albums.Albums[0].ID.String()

	if cachedAlbum, exists := s.albumCache[albumID]; exists {
		return cachedAlbum, nil
	}

	// Get full album details
	album, err := s.client.GetAlbum(ctx, spotify.ID(albumID))
	if err != nil {
		return nil, fmt.Errorf("failed to get album details for ID %s: %w", albumID, err)
	}

	// Clean cache if needed before adding new item
	s.cleanAlbumCache()

	// Cache the result
	s.albumCache[albumID] = album

	return album, nil
}
