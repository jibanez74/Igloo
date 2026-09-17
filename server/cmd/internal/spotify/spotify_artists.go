package spotify

import (
	"context"
	"strings"

	"github.com/zmb3/spotify/v2"
)

func (s *spotifyClient) SearchArtistByName(ctx context.Context, artistName string) (*spotify.FullArtist, error) {
	artistName = strings.TrimSpace(artistName)
	if artistName == "" {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:    lookupArtist,
			Input:     artistName,
			Strategy:  strategyArtistSearch,
			Reason:    MatchReasonEmptyQuery,
			Threshold: spotifyArtistThreshold,
		}, nil)
	}

	cacheKey := strings.ToLower(artistName)

	if cached, exists := s.getArtist(cacheKey); exists {
		return cached, nil
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	results, err := s.client.Search(ctx, artistName, spotify.SearchTypeArtist, spotify.Limit(spotifyArtistSearchLimit))
	if err != nil {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:      lookupArtist,
			Input:       artistName,
			SearchQuery: artistName,
			Strategy:    strategyArtistSearch,
			Reason:      MatchReasonSearchFailed,
			Threshold:   spotifyArtistThreshold,
		}, err)
	}

	if results.Artists == nil {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:      lookupArtist,
			Input:       artistName,
			SearchQuery: artistName,
			Strategy:    strategyArtistSearch,
			Reason:      MatchReasonNoResults,
			Threshold:   spotifyArtistThreshold,
		}, nil)
	}

	returned, info := selectBestArtistMatch(artistName, results.Artists.Artists, strategyArtistSearch)
	if returned == nil {
		return nil, newMatchError(info, nil)
	}

	s.setArtist(cacheKey, returned)

	return returned, nil
}
