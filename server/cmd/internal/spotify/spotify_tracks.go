package spotify

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (s *spotifyClient) SearchTracks(ctx context.Context, title string) ([]FullTrack, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("track title is required")
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	results, err := s.search(ctx, title, "track", spotifyTrackRequestSearchLimit)
	if err != nil {
		return nil, fmt.Errorf("spotify track search failed: %w", err)
	}
	if results.Tracks == nil {
		return []FullTrack{}, nil
	}

	return results.Tracks.Items, nil
}
