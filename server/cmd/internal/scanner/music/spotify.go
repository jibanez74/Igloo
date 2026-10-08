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
	musicSpotifyEntityAlbum     = "album"
	musicSpotifyEntityMusician  = "musician"
	musicSpotifyStatusMatched   = "matched"
	musicSpotifyStatusFailed    = "failed"
	musicSpotifyStatusUnmatched = "unmatched"
)

type resolvedSpotifyMatch struct {
	status string
	reason sql.NullString
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
			EntityType: entityType,
			EntityID:   entityID,
			Status:     match.status,
			Reason:     match.reason,
		})
		if err != nil {
			return err
		}
	}

	cache.Set(cacheKey, entityID)
	return nil
}

func resolvedSpotifyMatchFromError(err error) resolvedSpotifyMatch {
	match := resolvedSpotifyMatch{status: musicSpotifyStatusFailed}
	matchErr, ok := spotifyapi.AsMatchError(err)
	if !ok {
		return match
	}
	info := matchErr.Info
	unmatched := musicSpotifyReasonIsUnmatched(info.Reason)
	if unmatched {
		match.status = musicSpotifyStatusUnmatched
	}
	match.reason = helpers.NullString(info.Reason)
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
	return reason == spotifyapi.MatchReasonNoResults ||
		reason == spotifyapi.MatchReasonScoreBelowThreshold ||
		reason == spotifyapi.MatchReasonEmptyQuery
}

func musicSpotifyReasonSplitsCompound(reason string) bool {
	return reason == spotifyapi.MatchReasonNoResults || reason == spotifyapi.MatchReasonScoreBelowThreshold
}

// generateMusicianSummary words the Spotify popularity and follower figures as
// a sentence. Spotify omits both for some artists, in which case there is
// nothing to say: a sentence built from zeros ("is an independent artist with
// 0 followers") would read as a claim about the artist rather than a gap. The
// same holds for either figure alone, so a missing one contributes no phrase.
func generateMusicianSummary(artist *spotifylib.FullArtist) string {
	followers := artist.Followers.Count
	if artist.Popularity == 0 && followers == 0 {
		return ""
	}

	var parts []string

	parts = append(parts, artist.Name)

	if len(artist.Genres) > 0 {
		maxGenres := min(len(artist.Genres), 3)
		genreStr := strings.Join(artist.Genres[:maxGenres], ", ")
		parts = append(parts, fmt.Sprintf("known for %s", genreStr))
	}

	// The follower figure hangs off the popularity phrase ("is a popular artist
	// with ..."); without one it carries the sentence itself.
	followersVerb := "with"

	pop := artist.Popularity
	switch {
	case pop == 0:
		followersVerb = "has"
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

	switch {
	case followers == 0:
	case followers >= 10_000_000:
		parts = append(parts, fmt.Sprintf("%s over %dM followers on Spotify", followersVerb, followers/1_000_000))
	case followers >= 1_000_000:
		parts = append(parts, fmt.Sprintf("%s %.1fM followers on Spotify", followersVerb, float64(followers)/1_000_000))
	case followers >= 100_000:
		parts = append(parts, fmt.Sprintf("%s %dK followers on Spotify", followersVerb, followers/1_000))
	case followers >= 1_000:
		parts = append(parts, fmt.Sprintf("%s %.1fK followers on Spotify", followersVerb, float64(followers)/1_000))
	default:
		parts = append(parts, fmt.Sprintf("%s %d followers on Spotify", followersVerb, followers))
	}

	return strings.Join(parts, " ") + "."
}

func firstImageURL(images []spotifylib.Image) string {
	if len(images) == 0 {
		return ""
	}

	return images[0].URL
}
