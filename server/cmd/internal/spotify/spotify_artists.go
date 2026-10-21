package spotify

import (
	"context"
	"strings"
)

func (s *spotifyClient) SearchArtistByName(ctx context.Context, artistName string) (*FullArtist, error) {
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

	results, err := s.search(ctx, artistName, "artist", spotifyArtistSearchLimit)
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

	// No artists page reads as no results, which selectBestArtistMatch reports.
	var artists []FullArtist
	if results.Artists != nil {
		artists = results.Artists.Items
	}
	returned, info := selectBestArtistMatch(artistName, artists, strategyArtistSearch)
	if returned == nil {
		return nil, newMatchError(info, nil)
	}

	s.setArtist(cacheKey, returned)

	return returned, nil
}
