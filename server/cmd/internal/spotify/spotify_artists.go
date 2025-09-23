package spotify

import (
	"context"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *SpotifyClient) GetArtistBySpotifyID(artistID string) (*spotify.FullArtist, error) {
	if cachedArtist, exists := s.artistCache[artistID]; exists {
		return cachedArtist, nil
	}

	ctx := context.Background()

	artist, err := s.client.GetArtist(ctx, spotify.ID(artistID))
	if err != nil {
		return nil, fmt.Errorf("failed to get artist with ID %s: %w", artistID, err)
	}

	// Clean cache if needed before adding new item
	s.cleanArtistCache()

	// Cache the result
	s.artistCache[artistID] = artist

	return artist, nil
}

func (s *SpotifyClient) SearchArtistByName(artistName string) (*spotify.FullArtist, error) {
	ctx := context.Background()

	results, err := s.client.Search(ctx, artistName, spotify.SearchTypeArtist, spotify.Limit(1))
	if err != nil {
		return nil, fmt.Errorf("failed to search for artist '%s': %w", artistName, err)
	}

	if len(results.Artists.Artists) == 0 {
		return nil, fmt.Errorf("no artists found for name '%s'", artistName)
	}

	artist := &results.Artists.Artists[0]

	// Clean cache if needed before adding new item
	s.cleanArtistCache()

	// Cache the result using the artist ID as key
	s.artistCache[artist.ID.String()] = artist

	return artist, nil
}
