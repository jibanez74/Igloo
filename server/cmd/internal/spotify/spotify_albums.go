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
			Lookup:    lookupAlbum,
			Input:     title,
			Strategy:  strategyAlbumFieldSearch,
			Reason:    MatchReasonEmptyQuery,
			Threshold: spotifyAlbumThreshold,
		}, nil)
	}

	cacheKey := strings.ToLower(title) + "|" + strings.ToLower(artist)

	if cached, exists := s.getAlbum(cacheKey); exists {
		return cached, nil
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	searchTitle := sanitizeSpotifySearchQueryValue(trimAlbumSearchTitle(title))
	searchArtist := sanitizeSpotifySearchQueryValue(artist)
	fieldQuery := fmt.Sprintf("album:\"%s\"", searchTitle)
	fallbackQuery := searchTitle
	if artist != "" {
		fieldQuery += fmt.Sprintf(" artist:\"%s\"", searchArtist)
		fallbackQuery += " " + searchArtist
	}

	bestInfo := MatchDebugInfo{
		Lookup:      lookupAlbum,
		Input:       title,
		SearchQuery: fieldQuery,
		Strategy:    strategyAlbumFieldSearch,
		Threshold:   spotifyAlbumThreshold,
		Reason:      MatchReasonNoResults,
	}

	// The field search runs first; a plain-text search is the fallback when it
	// finds no acceptable album.
	searches := []struct{ query, strategy string }{
		{fieldQuery, strategyAlbumFieldSearch},
		{fallbackQuery, strategyAlbumFallback},
	}
	for _, search := range searches {
		info := MatchDebugInfo{
			Lookup:      lookupAlbum,
			Input:       title,
			SearchQuery: search.query,
			Strategy:    search.strategy,
			Threshold:   spotifyAlbumThreshold,
		}

		results, err := s.client.Search(ctx, search.query, spotify.SearchTypeAlbum, spotify.Limit(spotifyAlbumSearchLimit))
		if err != nil {
			info.Reason = MatchReasonSearchFailed
			return nil, newMatchError(info, err)
		}
		if results.Albums == nil {
			continue
		}

		returned, matchInfo := selectBestAlbumMatch(title, artist, results.Albums.Albums, search.query, search.strategy)
		bestInfo = chooseBetterMatchInfo(bestInfo, matchInfo)
		if returned == nil {
			continue
		}

		album, err := s.client.GetAlbum(ctx, returned.ID)
		if err != nil {
			info.CandidateName = returned.Name
			info.Reason = MatchReasonDetailsFailed
			return nil, newMatchError(info, err)
		}

		s.setAlbum(cacheKey, album)
		return album, nil
	}

	return nil, newMatchError(bestInfo, nil)
}
