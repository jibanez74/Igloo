package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// spotifyAlbumTracksPageLimit is the most tracks one album-tracks request
// returns.
const spotifyAlbumTracksPageLimit = 50

func (c *Client) SearchAlbums(ctx context.Context, title string) ([]SimpleAlbum, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("album title is required")
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	results, err := c.search(ctx, title, "album", spotifyAlbumRequestSearchLimit)
	if err != nil {
		return nil, fmt.Errorf("spotify album search failed: %w", err)
	}
	if results.Albums == nil {
		return []SimpleAlbum{}, nil
	}

	return results.Albums.Items, nil
}

// MatchAlbum finds the Spotify album for a local one and loads its details,
// track list included. The field search runs first; a plain-text search is
// the fallback when it finds no acceptable album.
func (c *Client) MatchAlbum(ctx context.Context, query AlbumQuery) (*FullAlbum, error) {
	query.Title = strings.TrimSpace(query.Title)
	query.Artist = strings.TrimSpace(query.Artist)
	if query.Title == "" {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:    lookupAlbum,
			Input:     query.Title,
			Strategy:  strategyAlbumFieldSearch,
			Reason:    MatchReasonEmptyQuery,
			Threshold: spotifyAlbumThreshold,
		}, nil)
	}

	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	searchTitle := sanitizeSpotifySearchQueryValue(trimAlbumSearchTitle(query.Title))
	searchArtist := sanitizeSpotifySearchQueryValue(query.Artist)
	fieldQuery := fmt.Sprintf("album:\"%s\"", searchTitle)
	fallbackQuery := searchTitle
	if query.Artist != "" {
		fieldQuery += fmt.Sprintf(" artist:\"%s\"", searchArtist)
		fallbackQuery += " " + searchArtist
	}

	bestInfo := MatchDebugInfo{
		Lookup:      lookupAlbum,
		Input:       query.Title,
		SearchQuery: fieldQuery,
		Strategy:    strategyAlbumFieldSearch,
		Threshold:   spotifyAlbumThreshold,
		Reason:      MatchReasonNoResults,
	}

	searches := []struct{ query, strategy string }{
		{fieldQuery, strategyAlbumFieldSearch},
		{fallbackQuery, strategyAlbumFallback},
	}
	for _, search := range searches {
		info := MatchDebugInfo{
			Lookup:      lookupAlbum,
			Input:       query.Title,
			SearchQuery: search.query,
			Strategy:    search.strategy,
			Threshold:   spotifyAlbumThreshold,
		}

		results, err := c.search(ctx, search.query, "album", spotifyAlbumSearchLimit)
		if err != nil {
			info.Reason = MatchReasonSearchFailed
			return nil, newMatchError(info, err)
		}
		if results.Albums == nil {
			continue
		}

		returned, matchInfo := selectBestAlbumMatch(query, results.Albums.Items, search.query, search.strategy)
		bestInfo = chooseBetterMatchInfo(bestInfo, matchInfo)
		if returned == nil {
			continue
		}

		album, err := c.GetAlbum(ctx, returned.ID)
		if err != nil {
			info.CandidateName = returned.Name
			info.Reason = MatchReasonDetailsFailed
			return nil, newMatchError(info, err)
		}

		return album, nil
	}

	return nil, newMatchError(bestInfo, nil)
}

// GetAlbum loads an album by id with every page of its track list.
func (c *Client) GetAlbum(ctx context.Context, id ID) (*FullAlbum, error) {
	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	albumURL := spotifyAPIBaseURL + "/albums/" + url.PathEscape(id.String())
	var details albumDetails
	err := c.getJSON(ctx, albumURL, &details)
	if err != nil {
		return nil, fmt.Errorf("spotify album lookup failed: %w", err)
	}

	tracks := details.Tracks.Items
	total := int(details.Tracks.Total)
	for len(tracks) < total {
		params := url.Values{
			"limit":  {strconv.Itoa(spotifyAlbumTracksPageLimit)},
			"offset": {strconv.Itoa(len(tracks))},
		}
		var page trackPage
		err = c.getJSON(ctx, albumURL+"/tracks?"+params.Encode(), &page)
		if err != nil {
			return nil, fmt.Errorf("spotify album tracks failed: %w", err)
		}
		// An empty page means the total overstated the list; stop rather than
		// ask for the same offset forever.
		if len(page.Items) == 0 {
			break
		}
		tracks = append(tracks, page.Items...)
	}

	return &FullAlbum{SimpleAlbum: details.SimpleAlbum, Tracks: tracks}, nil
}
