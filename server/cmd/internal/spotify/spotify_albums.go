package spotify

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zmb3/spotify/v2"
)

func (s *spotifyClient) SearchAlbums(ctx context.Context, title string) ([]spotify.SimpleAlbum, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("album title is required")
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	results, err := s.client.Search(ctx, title, spotify.SearchTypeAlbum, spotify.Limit(spotifyAlbumRequestSearchLimit))
	if err != nil {
		return nil, fmt.Errorf("spotify album search failed: %w", err)
	}
	if results.Albums == nil {
		return []spotify.SimpleAlbum{}, nil
	}

	return results.Albums.Albums, nil
}

func (s *spotifyClient) SearchAndGetAlbumDetails(ctx context.Context, title, artist string) (*spotify.FullAlbum, error) {
	title = strings.TrimSpace(title)
	artist = strings.TrimSpace(artist)
	if title == "" {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:    "album",
			Input:     title,
			Strategy:  "album_field_search",
			Reason:    "empty_query",
			Threshold: spotifyAlbumThreshold,
		}, nil)
	}

	cacheKey := strings.ToLower(title) + "|" + strings.ToLower(artist)

	if cached, exists := s.getAlbum(cacheKey); exists {
		return cached, nil
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	searchTitle := trimAlbumSearchTitle(title)

	var query string
	if artist != "" {
		query = fmt.Sprintf("album:\"%s\" artist:\"%s\"", searchTitle, artist)
	} else {
		query = fmt.Sprintf("album:\"%s\"", searchTitle)
	}

	results, err := s.client.Search(ctx, query, spotify.SearchTypeAlbum, spotify.Limit(spotifyAlbumSearchLimit))
	if err != nil {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:      "album",
			Input:       title,
			SearchQuery: query,
			Strategy:    "album_field_search",
			Reason:      "search_failed",
			Threshold:   spotifyAlbumThreshold,
		}, err)
	}

	bestInfo := MatchDebugInfo{
		Lookup:      "album",
		Input:       title,
		SearchQuery: query,
		Strategy:    "album_field_search",
		Threshold:   spotifyAlbumThreshold,
		Reason:      "no_results",
	}

	if results.Albums != nil {
		returned, info := selectBestAlbumMatch(title, artist, results.Albums.Albums, query, "album_field_search")
		bestInfo = chooseBetterMatchInfo(bestInfo, info)
		if returned != nil {
			album, err := s.client.GetAlbum(ctx, returned.ID)
			if err != nil {
				return nil, newMatchError(MatchDebugInfo{
					Lookup:        "album",
					Input:         title,
					SearchQuery:   query,
					Strategy:      "album_field_search",
					CandidateName: returned.Name,
					Reason:        "details_failed",
					Threshold:     spotifyAlbumThreshold,
				}, err)
			}

			s.setAlbum(cacheKey, album)

			return album, nil
		}
	}

	var fallback string
	if artist != "" {
		fallback = searchTitle + " " + artist
	} else {
		fallback = searchTitle
	}

	results, err = s.client.Search(ctx, fallback, spotify.SearchTypeAlbum, spotify.Limit(spotifyAlbumSearchLimit))
	if err != nil {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:      "album",
			Input:       title,
			SearchQuery: fallback,
			Strategy:    "album_fallback_search",
			Reason:      "search_failed",
			Threshold:   spotifyAlbumThreshold,
		}, err)
	}

	if results.Albums == nil {
		return nil, newMatchError(bestInfo, nil)
	}

	returned, info := selectBestAlbumMatch(title, artist, results.Albums.Albums, fallback, "album_fallback_search")
	bestInfo = chooseBetterMatchInfo(bestInfo, info)
	if returned == nil {
		return nil, newMatchError(bestInfo, nil)
	}

	album, err := s.client.GetAlbum(ctx, returned.ID)
	if err != nil {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:        "album",
			Input:         title,
			SearchQuery:   fallback,
			Strategy:      "album_fallback_search",
			CandidateName: returned.Name,
			Reason:        "details_failed",
			Threshold:     spotifyAlbumThreshold,
		}, err)
	}

	s.setAlbum(cacheKey, album)

	return album, nil
}
