package spotify

import (
	"context"
	"errors"
	"fmt"

	"github.com/zmb3/spotify/v2"
)

func (s *SpotifyClient) GetArtistBySpotifyID(artistID string) (*spotify.FullArtist, error) {
	if artistID == "" {
		return nil, errors.New("artist ID cannot be empty")
	}

	ctx := context.Background()

	artist, err := s.client.GetArtist(ctx, spotify.ID(artistID))
	if err != nil {
		return nil, fmt.Errorf("failed to get artist with ID %s: %w", artistID, err)
	}

	return artist, nil
}

func (s *SpotifyClient) GetArtistAlbums(artistID string, limit int) ([]spotify.SimpleAlbum, error) {
	if artistID == "" {
		return nil, errors.New("artist ID cannot be empty")
	}

	if limit <= 0 {
		limit = 20
	}

	ctx := context.Background()

	albums, err := s.client.GetArtistAlbums(ctx, spotify.ID(artistID), []spotify.AlbumType{spotify.AlbumTypeAlbum, spotify.AlbumTypeSingle}, spotify.Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to get albums for artist with ID %s: %w", artistID, err)
	}

	return albums.Albums, nil
}

func (s *SpotifyClient) SearchArtists(query string, limit int) ([]spotify.FullArtist, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	if limit <= 0 {
		limit = 20 // default limit
	}

	ctx := context.Background()

	results, err := s.client.Search(ctx, query, spotify.SearchTypeArtist, spotify.Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to search artists for query '%s': %w", query, err)
	}

	return results.Artists.Artists, nil
}
