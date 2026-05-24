package spotify

import (
	"context"
	"fmt"
	"strings"

	"github.com/zmb3/spotify/v2"
)

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
			return s.getAndCacheAlbumDetails(ctx, cacheKey, returned, title, query, "album_field_search")
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
		fallbackInfo := MatchDebugInfo{
			Lookup:      "album",
			Input:       title,
			SearchQuery: fallback,
			Strategy:    "album_fallback_search",
			Threshold:   spotifyAlbumThreshold,
			Reason:      "no_results",
		}
		bestInfo = chooseBetterMatchInfo(bestInfo, fallbackInfo)
		return nil, newMatchError(bestInfo, nil)
	}

	returned, info := selectBestAlbumMatch(title, artist, results.Albums.Albums, fallback, "album_fallback_search")
	bestInfo = chooseBetterMatchInfo(bestInfo, info)
	if returned == nil {
		return nil, newMatchError(bestInfo, nil)
	}

	return s.getAndCacheAlbumDetails(ctx, cacheKey, returned, title, fallback, "album_fallback_search")
}

func (s *spotifyClient) getAndCacheAlbumDetails(
	ctx context.Context,
	cacheKey string,
	album *spotify.SimpleAlbum,
	title string,
	searchQuery string,
	strategy string,
) (*spotify.FullAlbum, error) {
	fullAlbum, err := s.client.GetAlbum(ctx, album.ID)
	if err != nil {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:        "album",
			Input:         title,
			SearchQuery:   searchQuery,
			Strategy:      strategy,
			CandidateName: album.Name,
			Reason:        "details_failed",
			Threshold:     spotifyAlbumThreshold,
		}, err)
	}

	s.setAlbum(cacheKey, fullAlbum)

	return fullAlbum, nil
}
