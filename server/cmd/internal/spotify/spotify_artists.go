package spotify

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *spotifyClient) SearchArtistByName(artistName string) (*spotify.FullArtist, error) {
	if artistName == "" {
		return nil, fmt.Errorf("artist name cannot be empty")
	}

	// Check cache first
	if cached, exists := s.getArtist(artistName); exists {
		return cached, nil
	}

	ctx := context.Background()

	results, err := s.client.Search(ctx, artistName, spotify.SearchTypeArtist, spotify.Limit(1))
	if err != nil {
		return nil, fmt.Errorf("failed to search for artist '%s': %w", artistName, err)
	}

	if len(results.Artists.Artists) == 0 {
		return nil, fmt.Errorf("no artists found for name '%s'", artistName)
	}

	artist := &results.Artists.Artists[0]

	// Store in cache
	s.setArtist(artistName, artist)

	return artist, nil
}
