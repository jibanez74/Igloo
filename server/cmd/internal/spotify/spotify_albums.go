package spotify

import (
	"context"
	"fmt"
	"strings"

	"github.com/zmb3/spotify/v2"
)

func (s *spotifyClient) SearchAndGetAlbumDetails(ctx context.Context, title, artist string) (*spotify.FullAlbum, error) {
	if title == "" {
		return nil, fmt.Errorf("album title cannot be empty")
	}

	cacheKey := strings.ToLower(strings.TrimSpace(title)) + "|" + strings.ToLower(strings.TrimSpace(artist))

	if cached, exists := s.getAlbum(cacheKey); exists {
		return cached, nil
	}

	// Strip release-type suffixes that Spotify omits from album titles.
	searchTitle := title
	for _, suffix := range []string{" - single", " - ep", " - lp", " - album"} {
		if idx := strings.Index(strings.ToLower(searchTitle), suffix); idx != -1 {
			searchTitle = strings.TrimSpace(searchTitle[:idx])
			break
		}
	}

	var query string
	if artist != "" {
		query = fmt.Sprintf("album:\"%s\" artist:\"%s\"", searchTitle, artist)
	} else {
		query = fmt.Sprintf("album:\"%s\"", searchTitle)
	}

	results, err := s.client.Search(ctx, query, spotify.SearchTypeAlbum, spotify.Limit(1))
	if err != nil {
		return nil, err
	}

	// If the field filter returned nothing (e.g. special characters broke the query),
	// fall back to a plain-text search.
	if len(results.Albums.Albums) == 0 {
		var fallback string
		if artist != "" {
			fallback = searchTitle + " " + artist
		} else {
			fallback = searchTitle
		}

		results, err = s.client.Search(ctx, fallback, spotify.SearchTypeAlbum, spotify.Limit(1))
		if err != nil {
			return nil, err
		}
	}

	if len(results.Albums.Albums) == 0 {
		return nil, fmt.Errorf("no albums found for query '%s'", query)
	}

	returned := results.Albums.Albums[0]

	// Normalize both sides: lowercase and strip non-alphanumeric characters to handle
	// apostrophe variants (smart vs straight quotes) and suffix differences like "- Single".
	normalize := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(s) {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == ' ' {
				b.WriteRune(r)
			}
		}
		return strings.TrimSpace(b.String())
	}

	normalizedReturned := normalize(returned.Name)
	normalizedTitle := normalize(title)
	if !strings.Contains(normalizedReturned, normalizedTitle) && !strings.Contains(normalizedTitle, normalizedReturned) {
		return nil, fmt.Errorf("spotify result '%s' does not match requested album '%s'", returned.Name, title)
	}

	album, err := s.client.GetAlbum(ctx, returned.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get album details for ID %s: %w", returned.ID, err)
	}

	s.setAlbum(cacheKey, album)

	return album, nil
}
