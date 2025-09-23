package spotify

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *SpotifyClient) SearchArtistByName(artistName string) (*spotify.FullArtist, error) {
	if artistName == "" {
		return nil, fmt.Errorf("artist name cannot be empty")
	}

	normalizedKey := normalizeCacheKey(artistName)

	cachedArtist, exists := s.artistCache[normalizedKey]
	if exists {
		return cachedArtist, nil
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
	s.cleanArtistCache()
	s.artistCache[normalizedKey] = artist

	return artist, nil
}
