package spotify

import (
	"context"
	"fmt"
	"strings"

	"github.com/zmb3/spotify/v2"
)

func (s *spotifyClient) SearchArtistByName(ctx context.Context, artistName string) (*spotify.FullArtist, error) {
	if artistName == "" {
		return nil, fmt.Errorf("artist name cannot be empty")
	}

	cacheKey := strings.ToLower(strings.TrimSpace(artistName))

	if cached, exists := s.getArtist(cacheKey); exists {
		return cached, nil
	}

	results, err := s.client.Search(ctx, artistName, spotify.SearchTypeArtist, spotify.Limit(1))
	if err != nil {
		return nil, err
	}

	if len(results.Artists.Artists) == 0 {
		return nil, fmt.Errorf("no artists found for name '%s'", artistName)
	}

	returned := &results.Artists.Artists[0]
	if !strings.Contains(strings.ToLower(returned.Name), strings.ToLower(strings.TrimSpace(artistName))) {
		return nil, fmt.Errorf("spotify result '%s' does not match requested artist '%s'", returned.Name, artistName)
	}

	s.setArtist(cacheKey, returned)

	return returned, nil
}
