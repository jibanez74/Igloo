package spotify

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

func (c *Client) SearchArtistByName(ctx context.Context, artistName string) (*FullArtist, error) {
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

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	results, err := c.search(ctx, artistName, "artist", spotifyArtistSearchLimit)
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

	return returned, nil
}

// GetArtist loads one artist by id.
func (c *Client) GetArtist(ctx context.Context, id ID) (*FullArtist, error) {
	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	var artist FullArtist
	err := c.getJSON(ctx, spotifyAPIBaseURL+"/artists/"+url.PathEscape(id.String()), &artist)
	if err != nil {
		return nil, fmt.Errorf("spotify artist lookup failed: %w", err)
	}

	return &artist, nil
}
