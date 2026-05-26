package spotify

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zmb3/spotify/v2"
)

type albumSearchStrategy struct {
	query                string
	name                 string
	requireTrackEvidence bool
}

func (s *spotifyClient) SearchAndGetAlbumDetails(ctx context.Context, input AlbumSearchInput) (*spotify.FullAlbum, error) {
	ctx, cancel := spotifyRequestContext(ctx)
	defer cancel()

	input.Title = strings.TrimSpace(input.Title)
	input.Artist = strings.TrimSpace(input.Artist)
	if input.Title == "" {
		return nil, newMatchError(MatchDebugInfo{
			Lookup:    "album",
			Input:     input.Title,
			Strategy:  "album_field_search",
			Reason:    "empty_query",
			Threshold: spotifyAlbumThreshold,
		}, nil)
	}

	cacheKey := albumCacheKey(input)
	if cached, exists := s.getAlbum(cacheKey); exists {
		return cached, nil
	}

	bestInfo := MatchDebugInfo{
		Lookup:    "album",
		Input:     input.Title,
		Strategy:  "album_field_search",
		Threshold: spotifyAlbumThreshold,
		Reason:    "no_results",
	}

	for _, strategy := range buildAlbumSearchStrategies(input) {
		results, err := s.client.Search(ctx, strategy.query, spotify.SearchTypeAlbum, spotify.Limit(spotifyAlbumSearchLimit))
		if err != nil {
			return nil, newMatchError(MatchDebugInfo{
				Lookup:      "album",
				Input:       input.Title,
				SearchQuery: strategy.query,
				Strategy:    strategy.name,
				Reason:      "search_failed",
				Threshold:   spotifyAlbumThreshold,
			}, err)
		}

		if results.Albums == nil {
			info := MatchDebugInfo{
				Lookup:      "album",
				Input:       input.Title,
				SearchQuery: strategy.query,
				Strategy:    strategy.name,
				Threshold:   spotifyAlbumThreshold,
				Reason:      "no_results",
			}
			bestInfo = chooseBetterMatchInfo(bestInfo, info)
			continue
		}

		returned, info := selectBestAlbumMatch(
			input.Title,
			input.Artist,
			input.Year,
			results.Albums.Albums,
			strategy.query,
			strategy.name,
		)
		if returned == nil {
			bestInfo = chooseBetterMatchInfo(bestInfo, info)
			continue
		}

		fullAlbum, err := s.getAlbumDetails(ctx, returned, input.Title, strategy.query, strategy.name)
		if err != nil {
			return nil, err
		}

		if albumMatchRequiresTrackEvidence(input, *returned, strategy) && !albumHasTrackEvidence(input.TrackTitles, fullAlbum) {
			info.Reason = "track_mismatch"
			bestInfo = chooseBetterMatchInfo(bestInfo, info)
			continue
		}

		bestInfo = chooseBetterMatchInfo(bestInfo, info)
		s.setAlbum(cacheKey, fullAlbum)

		return fullAlbum, nil
	}

	return nil, newMatchError(bestInfo, nil)
}

func albumCacheKey(input AlbumSearchInput) string {
	return strings.ToLower(input.Title) + "|" + strings.ToLower(input.Artist) + "|" + strconv.Itoa(input.Year)
}

func buildAlbumSearchStrategies(input AlbumSearchInput) []albumSearchStrategy {
	searchTitle := trimAlbumSearchTitle(input.Title)
	strategies := make([]albumSearchStrategy, 0, 4)

	if input.Artist != "" {
		strategies = append(strategies, albumSearchStrategy{
			query: fmt.Sprintf("album:\"%s\" artist:\"%s\"", searchTitle, input.Artist),
			name:  "album_field_search",
		})
		strategies = append(strategies, albumSearchStrategy{
			query: searchTitle + " " + input.Artist,
			name:  "album_fallback_search",
		})
	}

	strategies = append(strategies, albumSearchStrategy{
		query:                fmt.Sprintf("album:\"%s\"", searchTitle),
		name:                 "album_title_field_search",
		requireTrackEvidence: true,
	})
	strategies = append(strategies, albumSearchStrategy{
		query:                searchTitle,
		name:                 "album_title_fallback_search",
		requireTrackEvidence: true,
	})

	return strategies
}

func albumMatchRequiresTrackEvidence(input AlbumSearchInput, album spotify.SimpleAlbum, strategy albumSearchStrategy) bool {
	if len(input.TrackTitles) == 0 {
		return false
	}

	if strategy.requireTrackEvidence {
		return true
	}

	if input.Artist == "" {
		return true
	}

	artistScore, _ := scoreAlbumArtist(input.Artist, album.Artists)
	return artistScore < spotifyArtistThreshold
}

func albumHasTrackEvidence(trackTitles []string, album *spotify.FullAlbum) bool {
	for _, title := range trackTitles {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}

		for _, candidateTrack := range album.Tracks.Tracks {
			if scoreAlbumTitle(title, candidateTrack.Name) >= spotifyAlbumTrackThreshold {
				return true
			}
		}
	}

	return false
}

func (s *spotifyClient) getAlbumDetails(
	ctx context.Context,
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

	return fullAlbum, nil
}
