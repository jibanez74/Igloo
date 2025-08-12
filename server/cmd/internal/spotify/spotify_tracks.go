package spotify

import (
	"context"
	"errors"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *SpotifyClient) GetTrackBySpotifyID(trackID string) (*spotify.FullTrack, error) {
	if trackID == "" {
		return nil, errors.New("track ID cannot be empty")
	}

	ctx := context.Background()

	track, err := s.client.GetTrack(ctx, spotify.ID(trackID))
	if err != nil {
		return nil, fmt.Errorf("failed to get track with ID %s: %w", trackID, err)
	}

	return track, nil
}

func (s *SpotifyClient) SearchTracks(query string, limit int) ([]spotify.FullTrack, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	if limit <= 0 {
		limit = 20 // default limit
	}

	ctx := context.Background()

	results, err := s.client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to search tracks for query '%s': %w", query, err)
	}

	return results.Tracks.Tracks, nil
}

// SearchTracksByArtist searches for tracks by a specific artist
func (s *SpotifyClient) SearchTracksByArtist(artistName string, limit int) ([]spotify.FullTrack, error) {
	if artistName == "" {
		return nil, errors.New("artist name cannot be empty")
	}

	// Search with artist name in the query
	query := fmt.Sprintf("artist:%s", artistName)
	return s.SearchTracks(query, limit)
}

