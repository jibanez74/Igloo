package music

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"

	spotifylib "github.com/zmb3/spotify/v2"
)

const (
	musicSpotifyEntityAlbum               = "album"
	musicSpotifyEntityMusician            = "musician"
	musicSpotifyStatusMatched             = "matched"
	musicSpotifyStatusFailed              = "failed"
	musicSpotifyStatusUnmatched           = "unmatched"
	musicSpotifyReasonNoResults           = "no_results"
	musicSpotifyReasonScoreBelowThreshold = "score_below_threshold"
	musicSpotifyReasonEmpty               = "empty_query"
)

type resolvedSpotifyMatch struct {
	status          string
	spotifyID       sql.NullString
	reason          sql.NullString
	score           sql.NullInt64
	thresholdValue  sql.NullInt64
	candidateName   sql.NullString
	candidateArtist sql.NullString
	searchQuery     sql.NullString
	strategy        sql.NullString
	errorText       sql.NullString
}

func (s *Scanner) upsertMusicSpotifyMatchAndCacheID(
	ctx context.Context,
	qtx *database.Queries,
	entityType string,
	entityID int64,
	match *resolvedSpotifyMatch,
	cache scanner.ScanCache[string, int64],
	cacheKey string,
) error {
	if match != nil {
		err := qtx.UpsertMusicSpotifyMatch(ctx, database.UpsertMusicSpotifyMatchParams{
			EntityType:      entityType,
			EntityID:        entityID,
			SpotifyID:       match.spotifyID,
			Status:          match.status,
			Reason:          match.reason,
			Score:           match.score,
			ThresholdValue:  match.thresholdValue,
			CandidateName:   match.candidateName,
			CandidateArtist: match.candidateArtist,
			SearchQuery:     match.searchQuery,
			Strategy:        match.strategy,
			Error:           match.errorText,
		})
		if err != nil {
			return err
		}
	}

	cache.Set(cacheKey, entityID)
	return nil
}

func resolvedSpotifyMatchFromError(err error) resolvedSpotifyMatch {
	match := resolvedSpotifyMatch{
		status:    musicSpotifyStatusFailed,
		errorText: helpers.NullString(err.Error()),
	}

	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return match
	}

	info := matchErr.Info
	unmatched := musicSpotifyReasonIsUnmatched(info.Reason)
	if unmatched {
		match.status = musicSpotifyStatusUnmatched
		match.errorText = sql.NullString{}
	}

	match.reason = helpers.NullString(info.Reason)
	match.candidateName = helpers.NullString(info.CandidateName)
	match.candidateArtist = helpers.NullString(info.CandidateArtist)
	match.searchQuery = helpers.NullString(info.SearchQuery)
	match.strategy = helpers.NullString(info.Strategy)

	if info.Score > 0 {
		match.score = sql.NullInt64{Int64: int64(info.Score), Valid: true}
	}
	if info.Threshold > 0 {
		match.thresholdValue = sql.NullInt64{Int64: int64(info.Threshold), Valid: true}
	}
	if matchErr.Err != nil && match.status == musicSpotifyStatusFailed {
		match.errorText = helpers.NullString(matchErr.Err.Error())
	}

	return match
}

func musicSpotifyMatchSplitsCompound(status string, reason sql.NullString) bool {
	if status != musicSpotifyStatusUnmatched || !reason.Valid {
		return false
	}

	return musicSpotifyReasonSplitsCompound(reason.String)
}

func musicSpotifyMatchStatusIsFinal(status string) bool {
	return status == musicSpotifyStatusMatched || status == musicSpotifyStatusUnmatched
}

func musicSpotifyReasonIsUnmatched(reason string) bool {
	return reason == musicSpotifyReasonNoResults || reason == musicSpotifyReasonScoreBelowThreshold || reason == musicSpotifyReasonEmpty
}

func musicSpotifyReasonSplitsCompound(reason string) bool {
	return reason == musicSpotifyReasonNoResults || reason == musicSpotifyReasonScoreBelowThreshold
}

func generateMusicianSummary(artist *spotifylib.FullArtist) string {
	var parts []string

	parts = append(parts, artist.Name)

	if len(artist.Genres) > 0 {
		maxGenres := min(len(artist.Genres), 3)
		genreStr := strings.Join(artist.Genres[:maxGenres], ", ")
		parts = append(parts, fmt.Sprintf("known for %s", genreStr))
	}

	pop := artist.Popularity
	switch {
	case pop >= 80:
		parts = append(parts, "is a globally recognized artist")
	case pop >= 60:
		parts = append(parts, "is a popular artist")
	case pop >= 40:
		parts = append(parts, "has a dedicated following")
	case pop >= 20:
		parts = append(parts, "is an emerging artist")
	default:
		parts = append(parts, "is an independent artist")
	}

	followers := artist.Followers.Count
	switch {
	case followers >= 10_000_000:
		parts = append(parts, fmt.Sprintf("with over %dM followers on Spotify", followers/1_000_000))
	case followers >= 1_000_000:
		parts = append(parts, fmt.Sprintf("with %.1fM followers on Spotify", float64(followers)/1_000_000))
	case followers >= 100_000:
		parts = append(parts, fmt.Sprintf("with %dK followers on Spotify", followers/1_000))
	case followers >= 1_000:
		parts = append(parts, fmt.Sprintf("with %.1fK followers on Spotify", float64(followers)/1_000))
	default:
		parts = append(parts, fmt.Sprintf("with %d followers on Spotify", followers))
	}

	return strings.Join(parts, " ") + "."
}

func firstImageURL(images []spotifylib.Image) string {
	if len(images) == 0 {
		return ""
	}

	return images[0].URL
}
