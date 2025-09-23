package spotify

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *SpotifyClient) GetAlbumBySpotifyID(albumID string) (*spotify.FullAlbum, error) {
	cachedAlbum, exists := s.albumCache[albumID]
	if exists {
		return cachedAlbum, nil
	}

	ctx := context.Background()

	album, err := s.client.GetAlbum(ctx, spotify.ID(albumID))
	if err != nil {
		return nil, fmt.Errorf("failed to get album with ID %s: %w", albumID, err)
	}

	s.cleanAlbumCache()
	s.albumCache[albumID] = album

	return album, nil
}

func (s *SpotifyClient) SearchAlbums(query string, limit int) ([]spotify.SimpleAlbum, error) {
	if limit <= 0 {
		limit = 5
	}

	ctx := context.Background()

	results, err := s.client.Search(ctx, query, spotify.SearchTypeAlbum, spotify.Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to search albums for query '%s': %w", query, err)
	}

	albums := results.Albums.Albums

	return albums, nil
}
