package spotify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zmb3/spotify/v2"
)

func (s *spotifyClient) SearchTracks(ctx context.Context, title string) ([]spotify.FullTrack, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("track title is required")
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	results, err := s.client.Search(ctx, title, spotify.SearchTypeTrack, spotify.Limit(spotifyTrackRequestSearchLimit))
	if err != nil {
		return nil, fmt.Errorf("spotify track search failed: %w", err)
	}
	if results.Tracks == nil {
		return []spotify.FullTrack{}, nil
	}

	return results.Tracks.Tracks, nil
}
