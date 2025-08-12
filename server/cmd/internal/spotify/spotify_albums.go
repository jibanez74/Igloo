package spotify

import (
	"context"
	"errors"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *SpotifyClient) GetAlbumBySpotifyID(albumID string) (*spotify.FullAlbum, error) {
	if albumID == "" {
		return nil, errors.New("album ID cannot be empty")
	}

	ctx := context.Background()

	album, err := s.client.GetAlbum(ctx, spotify.ID(albumID))
	if err != nil {
		return nil, fmt.Errorf("failed to get album with ID %s: %w", albumID, err)
	}

	return album, nil
}

func (s *SpotifyClient) GetAlbumTracks(albumID string) ([]spotify.SimpleTrack, error) {
	if albumID == "" {
		return nil, errors.New("album id is required to fetch album tracks")
	}

	ctx := context.Background()

	tracks, err := s.client.GetAlbumTracks(ctx, spotify.ID(albumID))
	if err != nil {
		return nil, fmt.Errorf("failed to get tracks for album with ID %s: %w", albumID, err)
	}

	return tracks.Tracks, nil
}

func (s *SpotifyClient) SearchAlbums(query string, limit int) ([]spotify.SimpleAlbum, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	if limit <= 0 {
		limit = 20
	}

	ctx := context.Background()

	results, err := s.client.Search(ctx, query, spotify.SearchTypeAlbum, spotify.Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to search albums for query '%s': %w", query, err)
	}

	return results.Albums.Albums, nil
}
